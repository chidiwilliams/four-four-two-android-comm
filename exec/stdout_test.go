package exec_test

import (
	"fmt"
	"four-four-two-android-comm/exec"
	"testing"
)

func TestRunStdout(t *testing.T) {
	type args struct {
		out  chan string
		name string
		args []string
	}
	tests := []struct {
		name string
		args args
		err  error
	}{
		{
			name: "ls",
			args: args{
				out:  make(chan string),
				name: "ls",
				args: []string{},
			},
			err: nil,
		},
		// {
		// 	name: "MSFP",
		// 	args: args{
		// 		out:  make(chan string),
		// 		name: "./MSFP",
		// 		args: []string{"LEFT"},
		// 	},
		// 	err: nil,
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			go func() {
				defer close(tt.args.out)

				if err := exec.RunStdout(tt.args.out, tt.args.name, tt.args.args...); err != tt.err {
					t.Errorf("RunStdout() error expected %v, got %v", tt.err, err)
				}
			}()

			for x := range tt.args.out {
				fmt.Println(x)
			}
		})
	}
}
