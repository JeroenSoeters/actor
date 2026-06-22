package statemachine

import (
	"strings"
	"testing"

	"ergo.services/ergo/gen"
	"ergo.services/ergo/testing/unit"
)

type Data struct{}

type BasicStatemachine struct {
	StateMachine[Data]
}

type StateChange struct{}

func factoryBasicStatemachine() gen.ProcessBehavior {
	return &BasicStatemachine{}
}

func (b *BasicStatemachine) Init(args ...any) (StateMachineSpec[Data], error) {
	spec := NewStateMachineSpec(
		gen.Atom("StateA"),

		WithStateEnterCallback(stateEnter),
		WithStateMessageHandler(gen.Atom("StateA"), changeState),
		WithStateCallHandler(gen.Atom("StateA"), changeStateSync),
	)
	return spec, nil
}

func changeState(from gen.PID, state gen.Atom, data Data, msg StateChange, proc gen.Process) (gen.Atom, Data, []Action, error) {
	return gen.Atom("StateB"), data, nil, nil
}

func changeStateSync(from gen.PID, state gen.Atom, data Data, msg StateChange, proc gen.Process) (gen.Atom, Data, gen.Atom, []Action, error) {
	return gen.Atom("StateB"), data, gen.Atom("StateB"), nil, nil
}

func stateEnter(oldState gen.Atom, newState gen.Atom, data Data, proc gen.Process) (gen.Atom, Data, error) {
	if newState == gen.Atom("StateB") {
		return newState, data, gen.TerminateReasonNormal
	}
	return newState, data, nil
}

// logLevelForMessage returns the level the actor logged a given message at. The
// test logger renders calls via fmt.Sprintf, so structured key/value args are
// appended as a "%!(EXTRA ...)" suffix; strip it to match on the message text.
func logLevelForMessage(actor *unit.TestActor, message string) (gen.LogLevel, bool) {
	for _, e := range actor.Events() {
		le, ok := e.(unit.LogEvent)
		if !ok {
			continue
		}
		text := le.Message
		if i := strings.Index(text, "%!(EXTRA"); i != -1 {
			text = text[:i]
		}
		if text == message {
			return le.Level, true
		}
	}
	return 0, false
}

func TestDroppingStaleGenericTimeout_LogsAtDebug(t *testing.T) {
	actor, err := unit.Spawn(t, factoryBasicStatemachine, unit.WithLogLevel(gen.LogLevelDebug))
	unit.Nil(t, err)

	actor.SendMessage(
		gen.PID{Node: "test", ID: 100, Creation: 0},
		genericTimeoutMessage{name: gen.Atom("rate-limit"), generation: 1})

	level, found := logLevelForMessage(actor, "StateMachine: dropping stale generic timeout")
	unit.Equal(t, true, found)
	unit.Equal(t, gen.LogLevelDebug, level)
}

func TestDroppingStaleGenericTimeoutAfterReplacement_LogsAtDebug(t *testing.T) {
	actor, err := unit.Spawn(t, factoryBasicStatemachine, unit.WithLogLevel(gen.LogLevelDebug))
	unit.Nil(t, err)

	sm := actor.Behavior().(*BasicStatemachine)
	sm.genericTimeouts[gen.Atom("rate-limit")] = &ActiveGenericTimeout{generation: 2}

	actor.SendMessage(
		gen.PID{Node: "test", ID: 100, Creation: 0},
		genericTimeoutMessage{name: gen.Atom("rate-limit"), generation: 1})

	level, found := logLevelForMessage(actor, "StateMachine: dropping stale generic timeout after replacement")
	unit.Equal(t, true, found)
	unit.Equal(t, gen.LogLevelDebug, level)
}

func TestStateEnterCallback_SendShouldPropagateErrors(t *testing.T) {
	actor, err := unit.Spawn(t, factoryBasicStatemachine)
	unit.Nil(t, err)

	actor.SendMessage(
		gen.PID{Node: "test", ID: 100, Creation: 0},
		StateChange{})

	unit.Equal(t, true, actor.IsTerminated())
	unit.Equal(t, gen.TerminateReasonNormal, actor.TerminationReason())
}

func TestStateEnterCallback_CallShouldPropagateErrors(t *testing.T) {
	actor, err := unit.Spawn(t, factoryBasicStatemachine)
	unit.Nil(t, err)

	res := actor.Call(
		gen.PID{Node: "test", ID: 100, Creation: 0},
		StateChange{})

	unit.Equal(t, gen.TerminateReasonNormal, res.Error)
	unit.Equal(t, true, actor.IsTerminated())
	unit.Equal(t, gen.TerminateReasonNormal, actor.TerminationReason())
}
