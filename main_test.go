package main

import "testing"

func Test_invalidGoFile(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name          string
		args          args
		invalidGoFile bool
	}{

		{
			name:          "valid go file",
			args:          args{filename: "testing.go"},
			invalidGoFile: true,
		},
		{
			name:          "valid go file",
			args:          args{filename: "testing.txt"},
			invalidGoFile: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := includeFileInCoverage(tt.args.filename); got != tt.invalidGoFile {
				t.Errorf("invalidGoFile() = %v, invalidGoFile %v", got, tt.invalidGoFile)
			}
		})
	}
}
