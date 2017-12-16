package callback

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func init() {
	http.DefaultClient.Timeout = 3 * time.Second
	RegisterCallback("http", CallbackFunc(httpCallback))
}

func httpCallback(cb, value string) error {
	if !strings.HasPrefix(cb, "http://") || !strings.HasPrefix(cb, "https://") {
		return ErrNotSupport
	}

	r, err := http.Post(cb, "text/plain", bytes.NewBufferString(value))
	if err != nil {
		return err
	}
	if r.ContentLength > 1 {
		io.CopyN(ioutil.Discard, r.Body, r.ContentLength)
	}
	r.Body.Close()
	if 200 <= r.StatusCode && r.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("%s", r.Status)
}
