package s3

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/ecwid/gosnap/registry"
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

func (c s3registry) Head(key string) (map[string]string, error) {
	head, err := c.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, noSuchKeyErr(err)
	}
	data := map[string]string{
		"last-modified-unix": fmt.Sprint(head.LastModified.Unix()),
	}
	for key, value := range head.Metadata {
		if value != nil {
			data[key] = *value
		}
	}
	return data, nil
}

func (c s3registry) Pull(key string) (*registry.Object, error) {
	var data = new(registry.Object)
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
	data.Body = buf.Bytes()
	data.Data = map[string]string{
		"last-modified-unix": fmt.Sprint(output.LastModified.Unix()),
	}
	for key, value := range output.Metadata {
		if value != nil {
			data.Data[key] = *value
		}
	}
	return data, nil
}

func (c s3registry) Push(key string, object registry.Object) error {
	req := &s3.PutObjectInput{
		Bucket:   aws.String(c.bucket),
		Key:      aws.String(key),
		ACL:      aws.String(s3.BucketCannedACLPublicRead),
		Metadata: map[string]*string{},
	}
	if object.Body != nil {
		req.SetContentType(http.DetectContentType(object.Body))
		req.SetBody(bytes.NewReader(object.Body))
		req.SetContentLength(int64(len(object.Body)))
	}
	for k, v := range object.Data {
		value := v
		req.Metadata[k] = &value
	}
	_, err := c.s3.PutObject(req)
	return err
}
