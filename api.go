package main

import (
	"context"
	"encoding/base64"
	"github.com/machinebox/graphql"
	"log"
	"os"
)

type Api struct {
	client *graphql.Client
	logger *log.Logger
	debug bool
}

func (apiClient Api) sendRun(key, toolchain string, runCode []byte,  problem, contest string) (int, error) {
	mutation := graphql.NewRequest(`
		mutation ($toolchain: String!, $runCode: String!, $problem: String!, $contest: String!) {
			submitSimple(toolchain: $toolchain, runCode: $runCode, problem: $problem, contest: $contest) {
				id
			}
		}
	`)
	mutation.Var("toolchain", toolchain)
	mutation.Var("runCode", base64.StdEncoding.EncodeToString(runCode))
	mutation.Var("problem", problem)
	mutation.Var("contest", contest)
	if key != "" {
		mutation.Header.Set("X-Jjs-Auth", key)
	}
	var response struct {
		SubmitSimple struct {
			Id int
		}
	}
	err := apiClient.client.Run(context.Background(), mutation, &response)
	if err != nil {
		if apiClient.debug {
			apiClient.logger.Println("Error while sending run: " + err.Error())
		}
		return -1, err
	}
	return response.Run.Id, nil
}

func (apiClient Api) getApiVersion(key string) (string, error) {
	query := graphql.NewRequest(`
		query {
			apiVersion
		}
	`)
	if key != "" {
		query.Header.Set("X-Jjs-Auth", key)
	}
	var response struct {
		ApiVersion string
	}
	err := apiClient.client.Run(context.Background(), query, &response)
	if err != nil { // TODO: compare this error to error when bad cookie is set
		if apiClient.debug {
			apiClient.logger.Println("Error while authenticating: " + err.Error())
		}
		return "", err
	}
	return response.ApiVersion, nil
}

func (apiClient Api) authorize(login, password string) (string, error) {
	mutation := graphql.NewRequest(`
		mutation ($login: String!, $password: String!) {
 			authSimple (login: $login, password: $password) {
				data
			}
		}
	`)
	mutation.Var("login", login)
	mutation.Var("password", password)
	var response struct {
		AuthSimple struct {
			Data string
		}
	}
	err := apiClient.client.Run(context.Background(), mutation, &response)
	if err != nil {
		if apiClient.debug {
			apiClient.logger.Println("Error while authorizing: " + err.Error())
		}
		return "", err
	}
	return response.AuthSimple.Data, nil
}

func (apiClient Api) createUser(key, login, password string, groups []string) (string, error) {
	mutation := graphql.NewRequest(`
		mutation ($login: String!, $password: String!, $groups: [String!]!) {
			createUser(login: $login, password: $password, groups: $groups) {
				id
			}
		}
	`)
	mutation.Var("login", login)
	mutation.Var("password", password)
	mutation.Var("groups", groups)
	if key != "" {
		mutation.Header.Set("X-Jjs-Auth", key)
	}
	var response struct {
		CreateUser struct {
			Id string
		}
	}
	err := apiClient.client.Run(context.Background(), mutation, &response)
	if err != nil {
		if apiClient.debug {
			apiClient.logger.Println("Error while creating user: " + err.Error())
		}
		return "", err
	}
	return response.CreateUser.Id, nil
}

func initialize(apiURL string, logFile *os.File, debug bool) *Api {
	client := graphql.NewClient(apiURL)
	logger := log.New(logFile, "", log.LstdFlags)
	return &Api{client, logger, debug}
}
