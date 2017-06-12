package lislackapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func parseSlackApp(r *http.Request) (*SlackContext, error) {
	sc := SlackContext{}
	slackSecret := os.Getenv("***ENV_VAR_HOLD_SECRET***")

	err := r.ParseForm()
	if err != nil {
		return nil, err
	}

	if sc.Token = r.Form.Get("token"); len(sc.Token) == 0 {
		return nil, errors.New("No token!")
	}

	if sc.TeamDomain = r.Form.Get("team_domain"); len(sc.TeamDomain) == 0 {
		return nil, errors.New("No team!")
	}

	if sc.ChannelID = r.Form.Get("channel_id"); len(sc.ChannelID) == 0 {
		return nil, errors.New("No channel!")
	}

	if sc.UserName = r.Form.Get("user_name"); len(sc.UserName) == 0 {
		return nil, errors.New("No user!")
	}

	if sc.Command = r.Form.Get("command"); len(sc.Command) == 0 {
		return nil, errors.New("No command!")
	}

	if sc.ResponseURL = r.Form.Get("response_url"); len(sc.ResponseURL) == 0 {
		return nil, errors.New("No response url!")
	}

	if strings.Compare(sc.Token, slackSecret) != 0 {
		return nil, errors.New("Invalid token")
	}

	if sc.Command != "***/push2live slash command name***" {
		return nil, errors.New("Invalid command!")
	}

	origin := sc.TeamDomain + sc.ChannelID

	if origin != "***slack_domain_name+slack_channel_id***" {
		return nil, errors.New("Invalid origin!")
	}

	sc.Text = r.Form.Get("text")

	return &sc, nil
}

func parseGitDescribe(s string) string {
	r := strings.Split(s, "-")
	var t []string

	if len(r) >= 2 && r[1] != "0" {
		t = append(t, r[1]+" commits behind")
		t = append(t, "Last pushed to live: "+r[0])
	}

	return strings.Join(t, "\n")
}

func pushToLive(path string, sc *SlackContext) (string, error) {
	var resp []string
	resp = append(resp, "Deploying to Live...")
	go pushToLiveDelayed(path, sc)
	return strings.Join(resp, "\n"), nil
}

func pushToLiveDelayed(path string, sc *SlackContext) (string, error) {
	var cmdOut string
	var err error
	var args, resp []string
	tmp := make([]interface{}, 1)

	args = []string{"reset", "--hard"}
	cmdOut, err = runOneCommand(path, "git", args, "git reset error: ")
	if err != nil {
		tmp[0] = map[string]string{"text": cmdOut + err.Error()}
		slackDelayedResponse("fail", tmp, sc)
		return cmdOut, err
	}

	args = []string{"clean", "-df"}
	cmdOut, err = runOneCommand(path, "git", args, "git clean error: ")
	if err != nil {
		tmp[0] = map[string]string{"text": cmdOut + err.Error()}
		slackDelayedResponse("fail", tmp, sc)
		return cmdOut, err
	}

	args = []string{"checkout", "master"}
	cmdOut, err = runOneCommand(path, "git", args, "git checkout master error: ")
	if err != nil {
		tmp[0] = map[string]string{"text": cmdOut + err.Error()}
		slackDelayedResponse("fail", tmp, sc)
		return cmdOut, err
	}

	args = []string{"pull"}
	cmdOut, err = runOneCommand(path, "git", args, "git pull error: ")
	if err != nil {
		tmp[0] = map[string]string{"text": cmdOut + err.Error()}
		slackDelayedResponse("fail", tmp, sc)
		return cmdOut, err
	}

	args = []string{"-c", "git describe --long --match 'live*'"}
	cmdOut, err = runOneCommand(path, "/bin/sh", args, "git describe error: ")
	if err != nil {
		tmp[0] = map[string]string{"text": cmdOut + err.Error()}
		slackDelayedResponse("fail", tmp, sc)
		return cmdOut, err
	}

	tagInfo := parseGitDescribe(cmdOut)

	if len(tagInfo) > 0 {
		resp = append(resp, tagInfo)
		t := time.Now()
		tString := fmt.Sprintf("%d%02d%02d%02d%02d%02d",
			t.Year(), t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second())

		args := []string{"tag", "-a", "live_" + tString, "-m", "\"slack\""}
		cmdOut, err := runOneCommand(path, "git", args, "git tag error: ")
		if err != nil {
			tmp[0] = map[string]string{"text": cmdOut + err.Error()}
			slackDelayedResponse("fail", tmp, sc)
			return cmdOut, err
		}

		args = []string{"push", "origin", "--tags"}
		cmdOut, err = runOneCommand(path, "git", args, "git push tags error: ")
		if err != nil {
			tmp[0] = map[string]string{"text": cmdOut + err.Error()}
			slackDelayedResponse("fail", tmp, sc)
			return cmdOut, err
		}
		tString2 := fmt.Sprintf("Pushed to Live on %s %02d, %02d at %02d:%02d:%02d",
			t.Month(), t.Day(), t.Year(),
			t.Hour(), t.Minute(), t.Second())
		resp = append(resp, tString2)

		resp = append(resp, "Restarting both dev and live servers")

		// make restart.sh into /etc/sudoers.d/mybypass
		// so that sudo ./restart.sh doesn't require password input

		args = []string{"-c", "sudo ./restart.sh"}
		cmdOut, err = runOneCommand("***/path/to/docker-compose/projectname", "/bin/sh", args, "restart servers error: ")

		if err != nil {
			tmp[0] = map[string]string{"text": cmdOut + err.Error()}
			slackDelayedResponse("fail", tmp, sc)
			return cmdOut, err
		}
		// resp = append(resp, cmdOut)
		resp = append(resp, "Restarted both dev and live servers")
	} else {
		resp = append(resp, "Live is already at HEAD")
	}

	respText := strings.Join(resp, "\n")
	tmp[0] = map[string]string{"text": respText}
	slackDelayedResponse("success", tmp, sc)
	return respText, nil

}

func runOneCommand(path string, cmd string, args []string, msg string) (string, error) {
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
		return msg, err
	}

	log.Println(string(cmdOut))
	return string(cmdOut), nil
}

type SlackContext struct {
	Token       string
	TeamDomain  string
	ChannelID   string
	UserName    string
	Command     string
	Text        string
	ResponseURL string
}

type SlackResponse struct {
	Response_type string        `json:"response_type"`
	Text          string        `json:"text"`
	Attachments   []interface{} `json:"attachments"`
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func slackDelayedResponse(text string, slack []interface{}, sc *SlackContext) {
	srd := SlackResponse{"in_channel", text, slack}
	js, err := json.Marshal(srd)
	checkError(err)
	req := bytes.NewBuffer([]byte(js))
	resp, err := http.Post(sc.ResponseURL, "application/json", req)
	checkError(err)
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}

func SlackRoutes() {
	http.HandleFunc("***/slack/slashcommandname***", func(w http.ResponseWriter, r *http.Request) {
		sc, err := parseSlackApp(r)
		a2 := make([]interface{}, 1)
		if err != nil {
			a2[0] = map[string]string{"text": err.Error()}
			resp := SlackResponse{"in_channel", "fail", a2}
			js, err := json.Marshal(resp)
			checkError(err)
			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
			log.Printf("Failed processing hook! ('%s')", err)
			panic(err)
		} else {
			result, err2 := "", errors.New("Not run")

			var resp SlackResponse
			switch project := sc.ChannelID; project {
			case "***slack_channel_id***":
				result, err2 = pushToLive("***/path/to/project***", sc)
			}

			if err2 != nil {
				a2[0] = map[string]string{"text": result + err2.Error()}
				resp = SlackResponse{"in_channel", "fail", a2}
			} else {
				a2[0] = map[string]string{"text": result}
				resp = SlackResponse{"in_channel", "success", a2}
			}

			js, err := json.Marshal(resp)
			checkError(err)
			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
		}
	})

}
