package gosnap

import (
	"testing"
	"time"
)

func TestBaselineApproveUpdate(t *testing.T) {
	now := time.Now()
	baseline := &Approvals{
		Value: []Approval{
			{Ts: now.Unix(), Hash: hashString("1"), Approver: "u1"},
			{Ts: now.Unix(), Hash: hashString("2"), Approver: "u2"},
		},
	}
	t.Log(baseline.Value)
	time.Sleep(time.Millisecond * 1200)
	baseline.accept(Approval{
		Hash:     hashString("1"),
		Approver: "u3",
	})
	if baseline.Value[0].Ts <= now.Unix() {
		t.Error("approval ts not updated", baseline.Value[0])
	}
	if baseline.Value[0].Approver != "u3" {
		t.Error("approver not updated", baseline.Value[0])
	}
}

func TestBaselineApproveOverflow(t *testing.T) {
	MaxApprovals = 4
	now := time.Now()
	baseline := &Approvals{
		Value: []Approval{
			{Ts: now.Add(time.Second * 2).Unix(), Hash: hashString("2sec"), Approver: "u2"},
			{Ts: now.Add(time.Second * 5).Unix(), Hash: hashString("5sec"), Approver: "u5"},
			{Ts: now.Add(time.Second * 10).Unix(), Hash: hashString("10sec"), Approver: "u10"},
			{Ts: now.Add(-time.Second * 10).Unix(), Hash: hashString("1"), Approver: "u"},
		},
	}

	t.Log(baseline.Value[0])
	t.Log(baseline.Value[1])
	t.Log(baseline.Value[2])
	t.Log(baseline.Value[3])

	baseline.accept(Approval{
		Hash:     hashString("newhash"),
		Approver: "new_user",
	})

	if baseline.Value[0].Ts < now.Unix() {
		t.Error("approval ts not updated", baseline.Value)
	}
	if baseline.Value[0].Hash.String() != "newhash" {
		t.Error("approval hash not updated", baseline.Value)
	}
	if baseline.Value[0].Approver != "new_user" {
		t.Error("approver not updated", baseline.Value)
	}
}
