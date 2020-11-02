package paladin

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {

}

func teardown() {
	//mockserver.Close()
}

func TestOasis(t *testing.T) {
	var (
		testAppYAML         = "app.yml"
		testAppYAMLContent1 = "test: 123"
		testAppYAMLContent2 = "test: 321"
	)
	os.Setenv("APP_ID", "wx01")
	os.Setenv("ZONE", "hk01")
	os.Setenv("DEPLOY_ENV", "dev")
	//os.Setenv("DISCOVERY_NODES", "127.0.0.1:7171")

	oasis, err := NewOasis()
	if err != nil {
		t.Fatalf("new oasis error, %v", err)
	}
	value := oasis.Get(testAppYAML)
	if content, _ := value.String(); content != testAppYAMLContent1 {
		t.Fatalf("got app.yml unexpected value %s", content)
	}

	updates := oasis.WatchEvent(context.TODO(), testAppYAML)
	select {
	case <-updates:
	case <-time.After(time.Second * 3000):
	}
	value = oasis.Get(testAppYAML)
	if content, _ := value.String(); content != testAppYAMLContent2 {
		t.Fatalf("got app.yml unexpected updated value %s", content)
	}
}
