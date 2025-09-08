// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package auth

import (
	"net/url"
	"testing"
)

func TestResourceURLFromServerURL_JSParity(t *testing.T) {
	t.Run("remove fragments", func(t *testing.T) {
		got, err := ResourceURLFromServerURL("https://example.com/path#fragment")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://example.com/path" {
			t.Fatalf("got %q, want %q", got.String(), "https://example.com/path")
		}

		got, err = ResourceURLFromServerURL("https://example.com#fragment")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://example.com" && got.String() != "https://example.com/" {
			t.Fatalf("got %q, want https://example.com(/)", got.String())
		}

		got, err = ResourceURLFromServerURL("https://example.com/path?query=1#fragment")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://example.com/path?query=1" {
			t.Fatalf("got %q, want %q", got.String(), "https://example.com/path?query=1")
		}
	})

	t.Run("no fragment -> unchanged", func(t *testing.T) {
		cases := []string{
			"https://example.com",
			"https://example.com/path",
			"https://example.com/path?query=1",
		}
		for _, in := range cases {
			got, err := ResourceURLFromServerURL(in)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", in, err)
			}
			// url.String() may print with or without a trailing slash when the path is rooted; both are accepted.
			want := in
			if got.String() != want && !(want == "https://example.com" && got.String() == "https://example.com/") {
				t.Fatalf("got %q, want %q", got.String(), want)
			}
		}
	})

	t.Run("keep everything else unchanged (except fragment)", func(t *testing.T) {
		got, err := ResourceURLFromServerURL("https://example.com:443/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://example.com:443/path" {
			t.Fatalf("got %q, want %q", got.String(), "https://example.com:443/path")
		}

		got, err = ResourceURLFromServerURL("https://example.com:8080/path")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://example.com:8080/path" {
			t.Fatalf("got %q, want %q", got.String(), "https://example.com:8080/path")
		}

		got, err = ResourceURLFromServerURL("https://example.com/?foo=bar&baz=qux")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://example.com/?foo=bar&baz=qux" && got.String() != "https://example.com?foo=bar&baz=qux" {
			t.Fatalf("got %q, want https://example.com/?foo=bar&baz=qux", got.String())
		}

		got, err = ResourceURLFromServerURL("https://example.com/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://example.com/" {
			t.Fatalf("got %q, want %q", got.String(), "https://example.com/")
		}

		got, err = ResourceURLFromServerURL("https://example.com/path/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://example.com/path/" {
			t.Fatalf("got %q, want %q", got.String(), "https://example.com/path/")
		}
	})

	t.Run("unsupported type -> error", func(t *testing.T) {
		_, err := ResourceURLFromServerURL(123)
		if err == nil {
			t.Fatal("expected error for unsupported type")
		}
	})
}

func TestCheckResourceAllowed_JSParity(t *testing.T) {
	type args struct {
		req interface{}
		cfg interface{}
	}
	tests := []struct {
		name    string
		a       args
		want    bool
		wantErr bool
	}{
		{
			name: "identical URLs",
			a:    args{"https://example.com/path", "https://example.com/path"},
			want: true,
		},
		{
			name: "identical origins at root",
			a:    args{"https://example.com/", "https://example.com/"},
			want: true,
		},
		{
			name: "different paths -> false",
			a:    args{"https://example.com/path1", "https://example.com/path2"},
			want: false,
		},
		{
			name: "requested root vs configured path -> false",
			a:    args{"https://example.com/", "https://example.com/path"},
			want: false,
		},
		{
			name: "different domain -> false",
			a:    args{"https://example.com/path", "https://example.org/path"},
			want: false,
		},
		{
			name: "different port -> false",
			a:    args{"https://example.com:8080/path", "https://example.com/path"},
			want: false,
		},
		{
			name: "path prefix but not by segment (mcpxxxx vs mcp) -> false",
			a:    args{"https://example.com/mcpxxxx", "https://example.com/mcp"},
			want: false,
		},
		{
			name: "requested shorter than configured -> false",
			a:    args{"https://example.com/folder", "https://example.com/folder/subfolder"},
			want: false,
		},
		{
			name: "requested is subpath of configured -> true",
			a:    args{"https://example.com/api/v1", "https://example.com/api"},
			want: true,
		},
		{
			name: "trailing slash handling: requested has slash, configured no slash -> true",
			a:    args{"https://example.com/mcp/", "https://example.com/mcp"},
			want: true,
		},
		{
			name: "trailing slash handling: requested no slash, configured has slash -> false (requested shorter)",
			a:    args{"https://example.com/folder", "https://example.com/folder/"},
			want: false,
		},
		{
			name:    "invalid requested URL -> error",
			a:       args{"https://%zz", "https://example.com/path"},
			wantErr: true,
		},
		{
			name:    "invalid configured URL -> error",
			a:       args{"https://example.com/path", "://bad_url"},
			wantErr: true,
		},
		{
			name:    "unsupported requested type -> error",
			a:       args{123, "https://example.com/path"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckResourceAllowed(CheckResourceAllowedParams{
				RequestedResource:  tt.a.req,
				ConfiguredResource: tt.a.cfg,
			})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (got=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceURLFromServerURL_URLInput(t *testing.T) {
	u, _ := url.Parse("https://example.com/A/B#frag")
	got, err := ResourceURLFromServerURL(u)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Fragment != "" {
		t.Fatalf("fragment not removed: %q", got.Fragment)
	}
	if got.Path != "/A/B" {
		t.Fatalf("path changed: %q", got.Path)
	}
}
