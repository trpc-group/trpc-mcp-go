// Copyright (c) 2024, Tencent Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type Task struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

func main() {
	server := flag.String("server", "http://localhost:8080", "server address")
	user := flag.String("user", "", "user id")
	cmd := flag.String("cmd", "list", "command: add/list/done/delete")
	content := flag.String("content", "", "task content (for add)")
	id := flag.Int("id", 0, "task id (for done/delete)")
	flag.Parse()

	if *user == "" {
		fmt.Println("user is required, use -user=xxx")
		os.Exit(1)
	}

	switch *cmd {
	case "add":
		if *content == "" {
			fmt.Println("content is required for add")
			os.Exit(1)
		}
		resp, err := http.PostForm(fmt.Sprintf("%s/task/add?user=%s", *server, *user), map[string][]string{"content": {*content}})
		if err != nil {
			fmt.Println("add failed:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))
	case "list":
		resp, err := http.Get(fmt.Sprintf("%s/task/list?user=%s", *server, *user))
		if err != nil {
			fmt.Println("list failed:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		var tasks []Task
		if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
			fmt.Println("decode failed:", err)
			os.Exit(1)
		}
		if len(tasks) == 0 {
			fmt.Println("No tasks.")
			return
		}
		for _, t := range tasks {
			status := "[ ]"
			if t.Done {
				status = "[x]"
			}
			fmt.Printf("%s %d: %s\n", status, t.ID, t.Content)
		}
	case "done":
		if *id == 0 {
			fmt.Println("id is required for done")
			os.Exit(1)
		}
		resp, err := http.PostForm(fmt.Sprintf("%s/task/done?user=%s", *server, *user), map[string][]string{"id": {fmt.Sprintf("%d", *id)}})
		if err != nil {
			fmt.Println("done failed:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))
	case "delete":
		if *id == 0 {
			fmt.Println("id is required for delete")
			os.Exit(1)
		}
		resp, err := http.PostForm(fmt.Sprintf("%s/task/delete?user=%s", *server, *user), map[string][]string{"id": {fmt.Sprintf("%d", *id)}})
		if err != nil {
			fmt.Println("delete failed:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))
	default:
		fmt.Println("unknown command, use add/list/done/delete")
		os.Exit(1)
	}
}
