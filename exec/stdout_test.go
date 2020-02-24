package exec

import (
	"fmt"
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
				name: "echo",
				args: []string{"Hello"},
			},
			err: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			go func() {
				err := RunStdout(tt.args.out, tt.args.name, tt.args.args...)
				if err != nil {
					t.Errorf("RunStdout() error = %v, err %v", err, tt.err)
				}

				close(tt.args.out)
			}()

			for x := range tt.args.out {
				fmt.Println(x)
			}
		})
	}
}
