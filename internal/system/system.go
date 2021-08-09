/*
    Copyright 2021 Rabia Research Team and Developers

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
*/
/*
	The package system contains only one function, which should be instantiated as a Go routine: it listens to OS
	signals like SIGINT and SIGTEM, and when it receives a signal, it closes the channel given as the sole argument to
	notify the caller routine. The use case is when a user wants to terminate Rabia (in case Rabia errs) before it
	normally exits.
*/
package system

import (
	"os"
	"os/signal"
	"syscall"
)

/*
	When SigListen is spawned as a Go routine by a caller routine, SigListen listens to SIGINT and SIGTERM. So when a
	user presses ctrl+c or the OS sends a kill signal, SigListen closes the done channel to notify the caller routine
	to exit.

	done: a channel created by the caller routine, the typical usage is like:
		at the caller routine:
			// initialize the done channel
			done := make(chan struct{})
			go SigListen(done)
			select {
			case <- done: // A receive from a closed channel returns the zero value immediately
				return
			case ...
			}
*/
func SigListen(done chan struct{}) {
	sigIn := make(chan os.Signal, 1)
	signal.Notify(sigIn, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-done:
		signal.Stop(sigIn)
	case <-sigIn:
		signal.Stop(sigIn)
		close(done)
	}
}
