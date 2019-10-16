package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	releasesUrl = "https://api.github.com/repos/JLevconoks/k8ConsoleViewer/releases/latest"
	appName     = "k8ConsoleViewer"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "automatically update k8ConsoleViewer",
	Run:   runUpdateCmd,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

type GithubReleasesResponse struct {
	URL         string    `json:"url"`
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

type Asset struct {
	URL         string `json:"url"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
}

func runUpdateCmd(cmd *cobra.Command, args []string) {
	version, assetUrl := checkLatestReleaseVersion()

	if version == buildVersion {
		fmt.Printf("No new version found. Current: %v, Latest: %v", buildVersion, version)
		return
	}

	fmt.Println("Current version:", buildVersion)
	fmt.Println("Latest version:", version)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Would you like to update (y/n)? ")
		choice, err := reader.ReadString('\n')
		checkError(err)
		switch strings.TrimSpace(choice) {
		case "y":
			downloadAndApply(assetUrl, version)
			return
		case "n":
			fmt.Println("Exiting...")
			return
		}
	}
}

func downloadAndApply(assetUrl string, newVersion string) {
	path, _ := getAppDir()
	appName := getAppName()
	fullPath := path + string(os.PathSeparator) + appName

	request, err := http.NewRequest("GET", assetUrl, nil)
	checkError(err)
	request.Header.Add("Accept", "application/octet-stream")

	client := &http.Client{}
	fmt.Println("Downloading...", assetUrl)
	response, err := client.Do(request)
	checkError(err)
	defer response.Body.Close()

	newFilePath := path + string(os.PathSeparator) + appName + "." + newVersion
	file, err := os.Create(newFilePath)
	checkError(err)
	checkError(file.Chmod(0755))
	_, err = io.Copy(file, response.Body)
	checkError(err)
	checkError(file.Close())
	fmt.Println("Downloaded:", newFilePath)

	fmt.Println("Updating...")
	checkError(os.Rename(fullPath, fullPath+"_backup"))
	checkError(os.Rename(newFilePath, fullPath))
	fmt.Println("Done.")
}

func checkLatestReleaseVersion() (version string, assetUrl string) {
	fmt.Println("Getting latest info from ", releasesUrl)
	resp, err := http.Get(releasesUrl)
	checkError(err)
	defer resp.Body.Close()

	fmt.Println(resp.Status)
	bytes, err := ioutil.ReadAll(resp.Body)
	checkError(err)

	var githubResponse GithubReleasesResponse
	checkError(json.Unmarshal(bytes, &githubResponse))

	var asset Asset
	for _, a := range githubResponse.Assets {
		if a.Name == appName {
			asset = a
		}
	}

	if asset.Name == "" {
		fmt.Printf("No %v asset found in latest release in %v", appName, releasesUrl)
		os.Exit(1)
	}

	return githubResponse.TagName, asset.URL
}

func getAppName() string {
	path := os.Args[0]
	index := strings.LastIndex(path, string(os.PathSeparator))
	if index >= 0 {
		return path[index+1:]
	}
	return path
}

func checkError(err error) {
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
