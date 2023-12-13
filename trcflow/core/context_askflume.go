//go:build darwin || linux
// +build darwin linux

package core

import (
	"github.com/trimble-oss/tierceron-nute/mashupsdk"
)

type AskFlumeMessage struct {
	Id      int64
	Message string
	Type    string
}

type AskFlumeContext struct {
	GchatQueries chan *AskFlumeContext
	DFQueries    chan *AskFlumeContext
	DFAnswers    chan *AskFlumeContext
	GchatAnswers chan *AskFlumeContext
	Upsert       chan *mashupsdk.MashupDetailedElementBundle
	Close        bool
	FlowCase     string
	Query        *AskFlumeMessage
	Queries      []*AskFlumeMessage
}

var id int64

// Keeps track of ID value for number of queries processed
// Should match up with ID for Flumeworld.MashupDetailedElementLibrary
func GetId() int64 {
	id += 1
	return id - 1
}

// Initializes the AskFlumeContext and returns the
// initialized context
func InitAskFlume() (*AskFlumeContext, error) {
	gchat_queries := make(chan *AskFlumeContext)
	df_queries := make(chan *AskFlumeContext)
	df_ans := make(chan *AskFlumeContext)
	gchat_ans := make(chan *AskFlumeContext)
	upsert := make(chan *mashupsdk.MashupDetailedElementBundle)
	empty_query := &AskFlumeMessage{
		Id:      0,
		Message: "",
	}

	id = 0
	cxt := &AskFlumeContext{
		GchatQueries: gchat_queries,
		DFQueries:    df_queries,
		DFAnswers:    df_ans,
		GchatAnswers: gchat_ans,
		Upsert:       upsert,
		Close:        false,
		FlowCase:     "",
		Query:        empty_query,
	}
	return cxt, nil
}
