//go:build darwin || linux || windows
// +build darwin linux windows

package core

import (
	"github.com/trimble-oss/tierceron-nute-core/mashupsdk"
)

type AskFlumeMessage struct {
	ID      int64
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

// GetID - keeps track of ID value for number of queries processed
// Should match up with ID for Flumeworld.MashupDetailedElementLibrary
func GetID() int64 {
	id += 1
	return id - 1
}

// InitAskFlume - initializes the AskFlumeContext and returns the
// initialized context
func InitAskFlume() (*AskFlumeContext, error) {
	gchatQueries := make(chan *AskFlumeContext)
	dfQueries := make(chan *AskFlumeContext)
	dfAnswers := make(chan *AskFlumeContext)
	gchatAnswers := make(chan *AskFlumeContext)
	upsert := make(chan *mashupsdk.MashupDetailedElementBundle)
	emptyQuery := &AskFlumeMessage{
		ID:      0,
		Message: "",
	}

	id = 0
	cxt := &AskFlumeContext{
		GchatQueries: gchatQueries,
		DFQueries:    dfQueries,
		DFAnswers:    dfAnswers,
		GchatAnswers: gchatAnswers,
		Upsert:       upsert,
		Close:        false,
		FlowCase:     "",
		Query:        emptyQuery,
	}
	return cxt, nil
}
