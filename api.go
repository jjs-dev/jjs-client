package main

import (
	"context"
	"encoding/base64"
	"github.com/shurcooL/graphql"
	"log"
	"net/http"
)

type HeaderTransport struct {
	rt      http.RoundTripper
	authKey string
}

func (transport *HeaderTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if transport.authKey != "" {
		request.Header.Set("X-Jjs-Auth", transport.authKey)
	}
	return transport.rt.RoundTrip(request)
}

type Api struct {
	transport *HeaderTransport
	client *graphql.Client
	debug bool
}

func (apiClient Api) addKeyToTransport(key string) {
	apiClient.transport.authKey = key
}

func (apiClient Api) sendRun(runCode, toolchain, problem, contest string) (int, error) {
	var mutation struct {
		SubmitSimple struct {
			Id int
		} `graphql:"submitSimple(toolchain: $toolchain, runCode: $runCode, problem: $problem, contest: $contest)"`
	}
	variables := map[string]interface{} {
		"toolchain": graphql.String(toolchain),
		"runCode": graphql.String(base64.StdEncoding.EncodeToString([]byte(runCode))),
		"problem": graphql.String(problem),
		"contest": graphql.String(contest),
	}
	err := apiClient.client.Mutate(context.Background(), &mutation, variables)
	if err != nil {
		if apiClient.debug {
			log.Println("Error while sending run: " + err.Error())
		}
		return -1, err
	}
	return mutation.SubmitSimple.Id, nil
}

func (apiClient Api) authenticate(key string) (bool, error) {
	var query struct {
		ApiVersion graphql.String
	}
	apiClient.transport.authKey = key
	err := apiClient.client.Query(context.Background(), &query, nil)
	apiClient.transport.authKey = ""
	if err != nil { // TODO: compare this error to error when bad cookie is set
		if apiClient.debug {
			log.Println("Error while authenticating: " + err.Error())
		}
		return false, err
	}
	return err == nil, nil
}

func (apiClient Api) authorize(login, password string) (string, error) {
	var mutation struct {
		SessionToken struct {
			Data string
		}`graphql:"authSimple(login: $login, password: $password)"`
	}
	variables := map[string]interface{} {
		"login": graphql.String(login),
		"password": graphql.String(password),
	}
	err := apiClient.client.Mutate(context.Background(), &mutation, variables)
	if err != nil {
		if apiClient.debug {
			log.Println("Error while authorizing: " + err.Error())
		}
		return "", err
	}
	return mutation.SessionToken.Data, nil
}

func (apiClient Api) createUser(login, password string, groups []string) (string, error) {
	var mutation struct {
		User struct {
			Id graphql.ID
		}`graphql:"createUser(login: $login, password: $password, groups: $groups)"`
	}
	var graphqlGroups []graphql.String
	for _, group := range groups {
		graphqlGroups = append(graphqlGroups, graphql.String(group))
	}
	variables := map[string]interface{} {
		"login": graphql.String(login),
		"password": graphql.String(password),
		"groups": graphqlGroups,
	}
	err := apiClient.client.Mutate(context.Background(), &mutation, variables)
	if err != nil {
		if apiClient.debug {
			log.Println("Error while creating user: " + err.Error())
		}
		return "", err
	}
	return mutation.User.Id.(string), nil
}

func initialize(apiURL string, debug bool) *Api {
	transport := HeaderTransport{rt: http.DefaultTransport, authKey: ""}
	httpClient := http.Client{Transport: &transport}
	client := graphql.NewClient(apiURL, &httpClient)
	return &Api{&transport, client, debug}
}
