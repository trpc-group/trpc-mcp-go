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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAddListDoneDeleteTask(t *testing.T) {
	// add
	req := httptest.NewRequest("POST", "/task/add?user=test", strings.NewReader(url.Values{"content": {"hello"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw := httptest.NewRecorder()
	addTaskHandler(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("addTaskHandler failed: %v", rw.Body.String())
	}

	// list
	req = httptest.NewRequest("GET", "/task/list?user=test", nil)
	rw = httptest.NewRecorder()
	listTaskHandler(rw, req)
	if !strings.Contains(rw.Body.String(), "hello") {
		t.Fatalf("listTaskHandler missing task: %v", rw.Body.String())
	}

	// done
	req = httptest.NewRequest("POST", "/task/done?user=test", strings.NewReader(url.Values{"id": {"1"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw = httptest.NewRecorder()
	doneTaskHandler(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("doneTaskHandler failed: %v", rw.Body.String())
	}

	// delete
	req = httptest.NewRequest("POST", "/task/delete?user=test", strings.NewReader(url.Values{"id": {"1"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rw = httptest.NewRecorder()
	deleteTaskHandler(rw, req)
	if rw.Code != http.StatusOK {
		t.Fatalf("deleteTaskHandler failed: %v", rw.Body.String())
	}
}
