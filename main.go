// Command api is the synthetic example/api service that Gawain's fixtures stand in for.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "api: the handlers and jobs are exercised by their tests; there is no server to start")
}
