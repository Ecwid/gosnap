package s3

import (
	"bytes"
	"fmt"
	"gosnap/registry"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	snapshotMaxSize = 14000
)

type s3registry struct {
	s3     *s3.S3
	bucket string
}

func NewRegistry(id, secret, bucket string) registry.Abstract {
	var value = s3registry{bucket: bucket}
	var sess, err = session.NewSession(&aws.Config{Region: aws.String(endpoints.UsEast1RegionID)})
	if err != nil {
		panic(err)
	}
	creds := credentials.NewStaticCredentials(id, secret, "")
	value.s3 = s3.New(sess, &aws.Config{Credentials: creds})
	return value
}

func (c s3registry) Resolve(key string) string {
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", c.bucket, key)
}

func noSuchKeyErr(err error) error {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case s3.ErrCodeNoSuchKey, "NotFound":
			return registry.ErrNoSuchKey
		}
	}
	return err
}

func (c s3registry) Pull(key string, downloadBody bool) (*registry.Object, error) {
	var (
		value    = new(registry.Object)
		metadata map[string]*string
	)
	if downloadBody {
		output, err := c.s3.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(c.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return nil, noSuchKeyErr(err)
		}
		var buf bytes.Buffer
		if _, err = io.Copy(&buf, output.Body); err != nil {
			return nil, err
		}
		value.Body = buf.Bytes()
		value.Last = *output.LastModified
		metadata = output.Metadata
	} else {
		head, err := c.s3.HeadObject(&s3.HeadObjectInput{
			Bucket: aws.String(c.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return nil, noSuchKeyErr(err)
		}
		metadata = head.Metadata
		value.Last = *head.LastModified
	}
	value.Data = map[string]string{}
	for key, val := range metadata {
		if val != nil {
			value.Data[key] = *val
		}
	}
	return value, nil
}

func (c s3registry) Push(key string, object registry.Object) error {
	// There is no defined limit on the total size of user metadata that can be applied to an object,
	// but a single HTTP request is limited to 16,000 bytes.
	// todo
	// if object.Data > snapshotMaxSize {
	// return errors.New("snapshot data is too big")
	// }
	req := &s3.PutObjectInput{
		Bucket:   aws.String(c.bucket),
		Key:      aws.String(key),
		ACL:      aws.String(s3.BucketCannedACLPublicRead),
		Metadata: map[string]*string{},
	}
	for key, value := range object.Data {
		req.Metadata[key] = &value
	}
	if object.Body != nil {
		req.SetContentType(http.DetectContentType(object.Body))
		req.SetBody(bytes.NewReader(object.Body))
		req.SetContentLength(int64(len(object.Body)))
	}
	_, err := c.s3.PutObject(req)
	return err
}
