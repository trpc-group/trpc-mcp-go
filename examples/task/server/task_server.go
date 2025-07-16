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
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
)

type Task struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

var (
	userTasks     = make(map[string][]Task)
	taskIDCounter = 0
	mu            sync.Mutex

	// SSE 相关
	userStreams = make(map[string]map[chan []Task]struct{}) // user -> set of channels
)

// SSE推送任务列表
func pushTaskUpdate(user string) {
	mu.Lock()
	streams := userStreams[user]
	tasks := append([]Task(nil), userTasks[user]...) // 拷贝，避免并发问题
	mu.Unlock()
	for ch := range streams {
		select {
		case ch <- tasks:
		default:
		}
	}
}

// SSE handler
func streamHandler(w http.ResponseWriter, r *http.Request) {
	user := getUserID(r)
	if user == "" {
		http.Error(w, "user required", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan []Task, 8)
	mu.Lock()
	if userStreams[user] == nil {
		userStreams[user] = make(map[chan []Task]struct{})
	}
	userStreams[user][ch] = struct{}{}
	mu.Unlock()
	defer func() {
		mu.Lock()
		delete(userStreams[user], ch)
		mu.Unlock()
		close(ch)
	}()

	// 初始推送一次
	mu.Lock()
	tasks := append([]Task(nil), userTasks[user]...)
	mu.Unlock()
	fmt.Fprintf(w, "data: %s\n\n", toJSON(tasks))
	flusher.Flush()

	// 持续推送
	for {
		select {
		case tasks, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", toJSON(tasks))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func getUserID(r *http.Request) string {
	return r.URL.Query().Get("user")
}

// 修改增删改接口，操作后推送SSE
func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	user := getUserID(r)
	if user == "" {
		http.Error(w, "user required", http.StatusBadRequest)
		return
	}
	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "content required", http.StatusBadRequest)
		return
	}
	mu.Lock()
	taskIDCounter++
	task := Task{ID: taskIDCounter, Content: content, Done: false}
	userTasks[user] = append(userTasks[user], task)
	mu.Unlock()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
	pushTaskUpdate(user)
}

func listTaskHandler(w http.ResponseWriter, r *http.Request) {
	user := getUserID(r)
	if user == "" {
		http.Error(w, "user required", http.StatusBadRequest)
		return
	}
	mu.Lock()
	tasks := userTasks[user]
	mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func doneTaskHandler(w http.ResponseWriter, r *http.Request) {
	user := getUserID(r)
	if user == "" {
		http.Error(w, "user required", http.StatusBadRequest)
		return
	}
	idStr := r.FormValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	mu.Lock()
	tasks := userTasks[user]
	for i, t := range tasks {
		if t.ID == id {
			tasks[i].Done = true
			break
		}
	}
	userTasks[user] = tasks
	mu.Unlock()
	w.Write([]byte("ok"))
	pushTaskUpdate(user)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	user := getUserID(r)
	if user == "" {
		http.Error(w, "user required", http.StatusBadRequest)
		return
	}
	idStr := r.FormValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	mu.Lock()
	tasks := userTasks[user]
	for i, t := range tasks {
		if t.ID == id {
			userTasks[user] = append(tasks[:i], tasks[i+1:]...)
			break
		}
	}
	mu.Unlock()
	w.Write([]byte("ok"))
	pushTaskUpdate(user)
}

func main() {
	http.HandleFunc("/task/add", addTaskHandler)
	http.HandleFunc("/task/list", listTaskHandler)
	http.HandleFunc("/task/done", doneTaskHandler)
	http.HandleFunc("/task/delete", deleteTaskHandler)
	http.HandleFunc("/task/stream", streamHandler) // 新增SSE接口
	fmt.Println("Task server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
