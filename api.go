package main

import (
    "context"
    "encoding/base64"
    "errors"
    "github.com/machinebox/graphql"
    "log"
    "os"
)

func setHeaders(request *graphql.Request, key string) {
    if key != "" {
        request.Header.Set("X-Jjs-Auth", key)
    }
    request.Header.Set("Connection", "close")
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
    setHeaders(mutation, key)
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
    return response.SubmitSimple.Id, nil
}

func (apiClient Api) getApiVersion(key string) (string, error) {
    query := graphql.NewRequest(`
        query {
            apiVersion
        }
    `)
    setHeaders(query, key)
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
    setHeaders(mutation, "")
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
    setHeaders(mutation, key)
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

func (apiClient* Api) listContests(key string) ([]Contest, error) { // TODO: pointer
    query := graphql.NewRequest(`
        query {
            contests {
                title
                id
                problems {
                    title
                    id
                }
            }
        }
    `)
    var response struct {
        Contests []Contest
    }
    setHeaders(query, key)
    err := apiClient.client.Run(context.Background(), query, &response)
    if err != nil {
        if apiClient.debug {
            apiClient.logger.Println("Error while listing contests: " + err.Error())
        }
        return []Contest{}, err
    }
    return response.Contests, nil
}


func (apiClient* Api) findContest(key, id string) (Contest, error) { // TODO: wait for JJS API Implementation
    contests, err := apiClient.listContests(key)
    if err != nil {
        return Contest{}, err
    }
    for _, contest := range contests {
        if contest.Id == id {
            return contest, nil
        }
    }
    return Contest{}, errors.New("not found contest")
}

func (apiClient *Api) ListToolChains(key string) ([]ToolChain, error) { // TODO: pointers
    query := graphql.NewRequest(`
        query {
            toolchains {
                name
                id
            }
        }
    `)
    var response struct {
        ToolChains []ToolChain
    }
    setHeaders(query, key)
    err := apiClient.client.Run(context.Background(), query, &response)
    if err != nil {
        if apiClient.debug {
            apiClient.logger.Println("Error while listing toolchains: " + err.Error())
        }
        return []ToolChain{}, err
    }
    return response.ToolChains, nil
}

func initialize(apiURL string, logFile *os.File, debug bool) *Api {
    client := graphql.NewClient(apiURL)
    logger := log.New(logFile, "", log.LstdFlags)
    return &Api{client, logger, debug}
}
