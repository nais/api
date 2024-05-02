package sqlinstance

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestSqlAdminService(t *testing.T) {
	ctx := context.Background()
	log := logrus.StandardLogger()
	sqladmin, err := NewSqlAdminService(ctx, log.WithField("test", "TestSqlAdminService_GetUsers"))
	if err != nil {
		t.Fatal(err)
	}

	instances, err := sqladmin.GetInstances(ctx, "tbd-dev-7ff9")
	if err != nil {
		t.Fatal(err)
	}

	for _, instance := range instances {
		t.Log("name:" + instance.Name)
		t.Log("project: " + instance.State)
	}

	users, err := sqladmin.GetUsers(ctx, "tbd-dev-7ff9", "sprute")
	if err != nil {
		t.Fatal(err)
	}

	for _, user := range users {
		t.Log("name:" + user.Name)
		t.Log("IAM type: " + user.Type)
	}
}
