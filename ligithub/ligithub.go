package ligithub

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type test_struct struct {
	Repository map[string]interface{}
}

type GitHubHookContext struct {
	Signature string
	Event     string
	Id        string
	Payload   []byte
}

type CIFolder struct {
	repo string
	Path string
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func signBody(secret, body []byte) []byte {
	computed := hmac.New(sha1.New, secret)
	computed.Write(body)
	return []byte(computed.Sum(nil))
}

func verifySecret(secret []byte, signature string, body []byte) bool {

	const signaturePrefix = "sha1="
	const signatureLength = 45 // len(SignaturePrefix) + len(hex(sha1))

	if len(signature) != signatureLength || !strings.HasPrefix(signature, signaturePrefix) {
		return false
	}

	actual := make([]byte, 20)
	hex.Decode(actual, []byte(signature[5:]))

	return hmac.Equal(signBody(secret, body), actual)
}

func parseGitHubHookContext(secret []byte, r *http.Request) (*GitHubHookContext, error) {

	hc := GitHubHookContext{}

	if hc.Signature = r.Header.Get("x-hub-signature"); len(hc.Signature) == 0 {
		return nil, errors.New("No signature!")
	}

	if hc.Event = r.Header.Get("x-github-event"); len(hc.Event) == 0 {
		return nil, errors.New("No event!")
	}

	if hc.Id = r.Header.Get("x-github-delivery"); len(hc.Id) == 0 {
		return nil, errors.New("No event Id!")
	}

	// log.Println("Signature: ", hc.Signature)
	// log.Println("Event: ", hc.Event)
	// log.Println("ID: ", hc.Id)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	if !verifySecret(secret, hc.Signature, body) {
		return nil, errors.New("Invalid signature")
	}

	hc.Payload = body
	return &hc, nil
}

func pullGitHub(path string) {
	var args []string
	args = []string{"reset", "--hard"}
	runCommands(path, "git", args, "git reset error: ")
	args = []string{"clean", "-df"}
	runCommands(path, "git", args, "git clean error: ")
	args = []string{"checkout", "master"}
	runCommands(path, "git", args, "git checkout master error: ")
	args = []string{"pull"}
	runCommands(path, "git", args, "git pull error: ")
}

func runCommands(path string, cmd string, args []string, msg string) {
	var (
		cmdOut []byte
		err    error
	)
	outputArgs := strings.Join(args, " ")
	log.Println("Running ", cmd, outputArgs)
	actual := exec.Command(cmd, args...)
	actual.Dir = path
	if cmdOut, err = actual.Output(); err != nil {
		log.Println(os.Stderr, msg, err)
		os.Exit(1)
	}

	log.Println(string(cmdOut))
}

func GithubRoutes() {
	http.HandleFunc("/payload", func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("***ENV_GITHUB_WEBHOOK_KEY***")
		hc, err := parseGitHubHookContext([]byte(secret), r)
		if err != nil {
			log.Printf("Failed processing hook! ('%s')", err)
			panic(err)
		} else {

			if hc.Event == "push" {
				//log.Println(string(hc.Payload))
				var t test_struct
				err = json.Unmarshal(hc.Payload, &t)
				checkError(err)

				switch reponame := t.Repository["full_name"]; reponame {
				case "***githubusername/gitreponame***":
					pullGitHub("***/path/to/project***")
				}

			}
		}

		fmt.Fprintf(w, "hello html")
		//out := fmt.Sprintf("%#v", r.URL)
		//fmt.Fprintf(w, out)
	})
}
