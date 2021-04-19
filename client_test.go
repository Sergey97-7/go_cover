package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// код писать тут
type TestCase struct {
	SearchErrorResponse
	SearchRequest
	SearchResponse
	IsError     bool
	AccessToken string
}
type Result struct {
	StatusCode          int
	Users               []User
	SearchErrorResponse SearchErrorResponse
}

var mockUsers = [...]User{
	{
		Id:     1,
		Name:   "Serj",
		Age:    23,
		About:  "123",
		Gender: "male",
	},
	{
		Id:     2,
		Name:   "Serj2",
		Age:    23,
		About:  "123456",
		Gender: "male",
	},
}

func FindUsersDummy(w http.ResponseWriter, r *http.Request) {
	option := r.Header.Get("AccessToken")

	switch option {
	case "ok":
		w.WriteHeader(http.StatusOK)
		res, _ := json.Marshal(mockUsers)
		w.Write(res)
	case "_bad_token":
		w.WriteHeader(http.StatusUnauthorized)
	case "_timeout_err":
		time.Sleep(2000 * time.Millisecond)
		w.WriteHeader(http.StatusGatewayTimeout)
	case "_fatal_err":
		w.WriteHeader(http.StatusInternalServerError)
	case "_bad_req_json_err":
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"}`)
	case "_bad_req_order_field_err":
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"Error": "ErrorBadOrderField"}`)
	case "_bad_req_unknown_err":
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"Error": "Something went wrong"}`)
	case "_internal_err":
		fallthrough
	default:
	}
}

func TestFindUsers(t *testing.T) {
	cases := []TestCase{
		{
			SearchRequest: SearchRequest{
				Limit:      3,
				Offset:     3,
				Query:      "Name",
				OrderField: "Name",
				OrderBy:    1,
			},
			SearchResponse: SearchResponse{
				Users:    mockUsers[0:2],
				NextPage: false,
			},
			IsError:     false,
			AccessToken: "ok",
		},
		{
			SearchRequest: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "Name",
				OrderField: "Name",
				OrderBy:    1,
			},
			SearchResponse: SearchResponse{
				Users:    mockUsers[0:1],
				NextPage: true,
			},
			IsError:     false,
			AccessToken: "ok",
		},
		{
			SearchRequest: SearchRequest{
				Limit:      26,
				Offset:     3,
				Query:      "Name",
				OrderField: "Name",
				OrderBy:    1,
			},
			SearchResponse: SearchResponse{
				Users:    mockUsers[0:2],
				NextPage: false,
			},
			IsError:     false,
			AccessToken: "ok",
		},
		// param errors
		{
			SearchRequest: SearchRequest{
				Limit:      -2,
				Offset:     0,
				Query:      "Name",
				OrderField: "Name",
				OrderBy:    1,
			},
			SearchResponse: SearchResponse{
				Users:    mockUsers[0:2],
				NextPage: false,
			},
			IsError:     true,
			AccessToken: "ok",
		},
		{
			SearchRequest: SearchRequest{
				Limit:      5,
				Offset:     -1,
				Query:      "Name",
				OrderField: "Name",
				OrderBy:    1,
			},
			SearchResponse: SearchResponse{
				Users:    mockUsers[0:2],
				NextPage: false,
			},
			IsError:     true,
			AccessToken: "ok",
		},
		//client errors
		{
			IsError:     true,
			AccessToken: "_timeout_err",
		},
		{
			IsError:     true,
			AccessToken: "_internal_err",
		},
		//http code errors
		{
			IsError:     true,
			AccessToken: "_bad_token",
		},
		{
			IsError:     true,
			AccessToken: "_fatal_err",
		},
		{
			IsError:     true,
			AccessToken: "_bad_req_json_err",
		},
		{
			IsError:     true,
			AccessToken: "_bad_req_order_field_err",
		},
		{
			IsError:     true,
			AccessToken: "_bad_req_unknown_err",
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(FindUsersDummy))

	for caseNum, item := range cases {
		client := &SearchClient{
			AccessToken: item.AccessToken,
			URL:         ts.URL,
		}
		res, err := client.FindUsers(item.SearchRequest)
		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", caseNum, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil: %#v", caseNum, err)
		}
		if err == nil && !reflect.DeepEqual(item.SearchResponse, *res) {
			t.Errorf("[%d] wrong result, expected %#v, got %#v", caseNum, item.SearchResponse, res)
		}
	}
	ts.Close()
}
func TestFindUsersTimeout(t *testing.T) {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Microsecond)
		w.WriteHeader(http.StatusOK)
		res, _ := json.Marshal(mockUsers)
		w.Write(res)
	})
	ts := httptest.NewServer(http.TimeoutHandler(handlerFunc, 1*time.Millisecond, "server timeout"))
	client := &SearchClient{
		AccessToken: "_timeout_err",
		URL:         ts.URL,
	}
	item := TestCase{
		IsError:     true,
		AccessToken: "_timeout_err",
	}
	http.DefaultTransport.(*http.Transport).ResponseHeaderTimeout = 10 * time.Millisecond
	time.Sleep(10 * time.Millisecond)

	http.DefaultTransport.(*http.Transport).ResponseHeaderTimeout = 50 * time.Millisecond
	res, err := client.FindUsers(item.SearchRequest)
	if err != nil && !item.IsError {
		t.Errorf("[%d] unexpected error: %#v", 1, err)
	}
	if err == nil && item.IsError {
		t.Errorf("[%d] expected error, got nil: %#v", 1, err)
	}
	if err == nil && !reflect.DeepEqual(item.SearchResponse, *res) {
		t.Errorf("[%d] wrong result, expected %#v, got %#v", 1, item.SearchResponse, res)
	}
	ts.Close()
}

func TestFindUsersHttpErr(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := &SearchClient{
		AccessToken: "_http_err",
		URL:         "",
	}
	item := TestCase{
		IsError:     true,
		AccessToken: "_http_err",
	}
	res, err := client.FindUsers(item.SearchRequest)

	if err != nil && !item.IsError {
		t.Errorf("[%d] unexpected error: %#v", 1, err)
	}
	if err == nil && item.IsError {
		t.Errorf("[%d] expected error, got nil: %#v", 1, err)
	}
	if err == nil && !reflect.DeepEqual(item.SearchResponse, *res) {
		t.Errorf("[%d] wrong result, expected %#v, got %#v", 1, item.SearchResponse, res)
	}
	ts.Close()
}
