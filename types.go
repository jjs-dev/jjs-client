package main

import (
    "github.com/machinebox/graphql"
    "html/template"
    "log"
)

type Api struct {
    client *graphql.Client
    logger *log.Logger
    debug bool
}

type Problem struct {
    Title string
    Id string
}

type Contest struct {
    Title string
    Id string
    Problems []Problem
}

type ToolChain struct {
    Name string
    Id string
}

type SimplePage struct {
    Message template.HTML
}

type ProblemPage struct {
    Contest *Contest
    ToolChains []*ToolChain
}
