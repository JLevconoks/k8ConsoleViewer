package version

import "fmt"

var (
	BuildVersion string = ""
	BuildTime    string = ""
)

func PrintVersion() {
	fmt.Println("Version:", BuildVersion)
	fmt.Println("Build Time:", BuildTime)
}
