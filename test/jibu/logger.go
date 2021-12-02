package jibu

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
)

// MyBy adds a timestamp before text
func MyBy(text string, callbacks ...func()) {
	text = fmt.Sprintf("%v %s", time.Now().Format("2006-01-02 15:04:05"), text)
	By(text, callbacks...)
}
