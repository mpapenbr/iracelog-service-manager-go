package processing

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/mpapenbr/iracelog-service-manager-go/pkg/model"
)

func ReadManifests(manifestFile string) (*model.Manifests, error) {
	// Open the JSON file
	file, err := os.Open(manifestFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the contents of the file
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into a model.Manifests struct
	var manifests model.Manifests
	err = json.Unmarshal(data, &manifests)
	if err != nil {
		return nil, err
	}

	return &manifests, nil
}

func TestProcessor_Process(t *testing.T) {
	manifests, _ := ReadManifests("./testdata/simple-manifests.json")

	type fields struct {
		CurrentData *model.AnalysisData
	}
	type args struct {
		stateMsg *model.StateData
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Processor{
				Manifests:   manifests,
				CurrentData: tt.fields.CurrentData,
			}
			p.ProcessState(tt.args.stateMsg)
		})
	}
}
