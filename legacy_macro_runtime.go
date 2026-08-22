package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
)

const (
	legacyMacroMaxStepsPerTick      = 64
	legacyMacroMaxStepsPerExecution = 10000
	legacyMacroMaxCallDepth         = 32
)

type legacyMacroRuntimeHooks struct {
	SendText     func(string)
	InsertText   func(string)
	SetText      func(string)
	Message      func(string)
	Move         func(legacyMacroMove)
	Complete     func(*legacyMacroExecution)
	RandomInt    func(int) int
	ResolveState func(string) (string, bool)
}

// legacyMacroMove is the native movement command accepted by the reference
// macro language. Direction zero is the explicit stop command.
type legacyMacroMove struct {
	Direction legacyMacroMoveDirection
	Run       bool
}

type legacyMacroMoveDirection uint8

const (
	legacyMacroMoveStop legacyMacroMoveDirection = iota
	legacyMacroMoveEast
	legacyMacroMoveNorthEast
	legacyMacroMoveNorth
	legacyMacroMoveNorthWest
	legacyMacroMoveWest
	legacyMacroMoveSouthWest
	legacyMacroMoveSouth
	legacyMacroMoveSouthEast
)

// legacyMacroRuntime owns variables and active executions for one loaded macro
// program. Its work is advanced by frames, so pause never blocks rendering or
// network processing.
type legacyMacroRuntime struct {
	mu sync.Mutex

	program   legacyMacroProgram
	globals   map[string]string
	functions map[string]int
	randoms   map[legacyMacroInstruction]int
	active    []*legacyMacroExecution

	sent        []string
	messages    []string
	diagnostics []legacyMacroDiagnostic
	hooks       legacyMacroRuntimeHooks
}

type legacyMacroExecution struct {
	kind      legacyMacroKind
	frames    []legacyMacroCallFrame
	variables map[string]string
	buffer    string
	result    string

	captureOutput bool
	lastLine      legacyMacroLine

	waitUntil  int64
	complete   bool
	diagnostic *legacyMacroDiagnostic
	steps      int
}

type legacyMacroCallFrame struct {
	declaration int
	nextLine    int
	elseIfLine  int
}

type legacyMacroInstruction struct {
	declaration int
	line        int
}

type legacyMacroControl uint8

const (
	legacyMacroControlNone legacyMacroControl = iota
	legacyMacroControlIf
	legacyMacroControlElseIf
	legacyMacroControlElse
	legacyMacroControlEndIf
	legacyMacroControlRandom
	legacyMacroControlOr
	legacyMacroControlEndRandom
	legacyMacroControlEnd
	legacyMacroControlLabel
	legacyMacroControlGoto
)

func newLegacyMacroRuntime(program legacyMacroProgram) *legacyMacroRuntime {
	return newLegacyMacroRuntimeWithHooks(program, legacyMacroRuntimeHooks{
		SendText:   legacyMacroQueueText,
		InsertText: legacyMacroInsertInputText,
		SetText:    legacyMacroSetInputText,
		Message:    consoleMessage,
		Move:       legacyMacroMovePlayer,
	})
}

func newLegacyMacroRuntimeWithHooks(program legacyMacroProgram, hooks legacyMacroRuntimeHooks) *legacyMacroRuntime {
	runtime := &legacyMacroRuntime{
		program:   program,
		globals:   make(map[string]string),
		functions: make(map[string]int),
		randoms:   make(map[legacyMacroInstruction]int),
		hooks:     hooks,
	}
	for index, declaration := range program.Macros {
		if declaration.Kind == legacyMacroFunction {
			if _, duplicate := runtime.functions[declaration.Trigger]; !duplicate {
				runtime.functions[declaration.Trigger] = index
			}
		}
	}
	for _, line := range program.TopLevel {
		if err := runtime.setVariableLocked(line, nil, true); err != nil {
			runtime.recordDiagnosticLocked(line, err.Error())
		}
	}
	return runtime
}

func legacyMacroQueueText(text string) {
	if text == "" {
		return
	}
	enqueueCommand(text)
	nextCommand()
}

func (runtime *legacyMacroRuntime) startFunction(name string) (*legacyMacroExecution, error) {
	return runtime.startFunctionWithContext(name, legacyMacroDefaultExecutionContext())
}

func (runtime *legacyMacroRuntime) startFunctionWithContext(name string, context legacyMacroExecutionContext) (*legacyMacroExecution, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	index, ok := runtime.functions[name]
	if !ok {
		return nil, fmt.Errorf("legacy macro function %q is not defined", name)
	}
	return runtime.startDeclarationWithContextLocked(index, context)
}

func (runtime *legacyMacroRuntime) startFunctionIfDefined(name string) *legacyMacroExecution {
	return runtime.startFunctionIfDefinedWithContext(name, legacyMacroDefaultExecutionContext())
}

func (runtime *legacyMacroRuntime) startFunctionIfDefinedWithContext(name string, context legacyMacroExecutionContext) *legacyMacroExecution {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	index, ok := runtime.functions[name]
	if !ok {
		return nil
	}
	execution, _ := runtime.startDeclarationWithContextLocked(index, context)
	return execution
}

func (runtime *legacyMacroRuntime) startDeclaration(index int) (*legacyMacroExecution, error) {
	return runtime.startDeclarationWithContext(index, legacyMacroDefaultExecutionContext())
}

func (runtime *legacyMacroRuntime) startDeclarationWithContext(index int, context legacyMacroExecutionContext) (*legacyMacroExecution, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.startDeclarationWithContextLocked(index, context)
}

func (runtime *legacyMacroRuntime) startDeclarationWithContextLocked(index int, context legacyMacroExecutionContext) (*legacyMacroExecution, error) {
	return runtime.startDeclarationWithOptionsLocked(index, context, false)
}

func (runtime *legacyMacroRuntime) startDeclarationWithOptionsLocked(index int, context legacyMacroExecutionContext, captureOutput bool) (*legacyMacroExecution, error) {
	if index < 0 || index >= len(runtime.program.Macros) {
		return nil, fmt.Errorf("legacy macro declaration %d does not exist", index)
	}
	declaration := runtime.program.Macros[index]
	execution := &legacyMacroExecution{
		kind:          declaration.Kind,
		frames:        []legacyMacroCallFrame{{declaration: index, elseIfLine: -1}},
		variables:     context.initialVariables(),
		captureOutput: captureOutput,
	}
	runtime.active = append(runtime.active, execution)
	return execution, nil
}

// advance runs each active macro until it pauses, sends one complete command,
// finishes, or uses its per-frame work budget.
func (runtime *legacyMacroRuntime) advance(frame int64) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	active := runtime.active[:0]
	for _, execution := range runtime.active {
		runtime.advanceExecutionLocked(execution, frame)
		if !execution.complete {
			active = append(active, execution)
		}
	}
	runtime.active = active
}

// interruptIfEnabled stops only executions that opt into the named legacy
// environment interrupt variable. The reference client checks this separately
// for each active execution, so a macro may choose to remain active.
func (runtime *legacyMacroRuntime) interruptIfEnabled(name string) int {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return runtime.interruptIfEnabledLocked(name)
}

func (runtime *legacyMacroRuntime) interruptIfEnabledLocked(name string) int {
	active := runtime.active[:0]
	interrupted := 0
	for _, execution := range runtime.active {
		value, enabled := runtime.variableLocked(name, execution)
		if enabled && strings.EqualFold(value, "true") {
			execution.complete = true
			interrupted++
			continue
		}
		active = append(active, execution)
	}
	runtime.active = active
	return interrupted
}

// cancelAll stops every active macro without flushing its buffered output.
func (runtime *legacyMacroRuntime) cancelAll() int {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	for _, execution := range runtime.active {
		execution.complete = true
	}
	interrupted := len(runtime.active)
	runtime.active = nil
	return interrupted
}

func (runtime *legacyMacroRuntime) advanceExecutionLocked(execution *legacyMacroExecution, frame int64) {
	if execution.complete {
		return
	}
	if execution.waitUntil > frame {
		return
	}
	execution.waitUntil = 0

	for step := 0; step < legacyMacroMaxStepsPerTick; step++ {
		if returnAt := strings.IndexByte(execution.buffer, '\r'); returnAt >= 0 {
			if execution.kind == legacyMacroReplacement {
				execution.buffer = execution.buffer[:returnAt]
				runtime.failExecutionLocked(execution, execution.lastLine, "replacement macros may not contain a return")
				return
			}
			runtime.sendTextLocked(execution, execution.buffer[:returnAt])
			execution.buffer = execution.buffer[returnAt+1:]
			return
		}

		for len(execution.frames) > 0 {
			current := execution.frames[len(execution.frames)-1]
			if current.nextLine < len(runtime.program.Macros[current.declaration].Body) {
				break
			}
			execution.frames = execution.frames[:len(execution.frames)-1]
		}
		if len(execution.frames) == 0 {
			runtime.completeExecutionLocked(execution)
			return
		}

		currentFrame := len(execution.frames) - 1
		current := &execution.frames[currentFrame]
		lineIndex := current.nextLine
		line := runtime.program.Macros[current.declaration].Body[lineIndex]
		current.nextLine++
		execution.lastLine = line
		execution.steps++
		if execution.steps > legacyMacroMaxStepsPerExecution {
			runtime.failExecutionLocked(execution, line, "execution limit exceeded")
			return
		}
		runtime.executeLineLocked(execution, currentFrame, lineIndex, line, frame)
		if execution.complete || execution.waitUntil > frame {
			return
		}
	}
}

func (runtime *legacyMacroRuntime) executeLineLocked(execution *legacyMacroExecution, currentFrame, lineIndex int, line legacyMacroLine, frame int64) {
	if len(line.Tokens) == 0 {
		return
	}
	first := line.Tokens[0]
	if first.Quote != 0 {
		runtime.appendTextLocked(execution, line.Tokens)
		return
	}

	switch legacyMacroLineControl(line) {
	case legacyMacroControlIf:
		runtime.executeIfLocked(execution, currentFrame, line, 1)
		return
	case legacyMacroControlElseIf:
		current := &execution.frames[currentFrame]
		if current.elseIfLine != lineIndex {
			runtime.skipToEndIfLocked(execution, currentFrame, line)
			return
		}
		current.elseIfLine = -1
		runtime.executeIfLocked(execution, currentFrame, line, 2)
		return
	case legacyMacroControlElse:
		runtime.skipToEndIfLocked(execution, currentFrame, line)
		return
	case legacyMacroControlEndIf, legacyMacroControlEndRandom, legacyMacroControlEnd, legacyMacroControlLabel:
		return
	case legacyMacroControlRandom:
		runtime.executeRandomLocked(execution, currentFrame, lineIndex, line)
		return
	case legacyMacroControlOr:
		runtime.skipToEndRandomLocked(execution, currentFrame, line)
		return
	case legacyMacroControlGoto:
		runtime.executeGotoLocked(execution, currentFrame, line)
		return
	}

	switch {
	case strings.EqualFold(first.Text, "pause"):
		if len(line.Tokens) != 2 {
			runtime.failExecutionLocked(execution, line, "pause requires one frame count")
			return
		}
		frames, err := strconv.Atoi(runtime.expandTokenLocked(line.Tokens[1], execution))
		if err != nil {
			runtime.failExecutionLocked(execution, line, "pause frame count must be a number")
			return
		}
		if frames > 0 {
			execution.waitUntil = frame + int64(frames)
		}
	case strings.EqualFold(first.Text, "set"):
		if err := runtime.setVariableLocked(line, execution, false); err != nil {
			runtime.failExecutionLocked(execution, line, err.Error())
		}
	case strings.EqualFold(first.Text, "setglobal"):
		if err := runtime.setVariableLocked(line, execution, true); err != nil {
			runtime.failExecutionLocked(execution, line, err.Error())
		}
	case strings.EqualFold(first.Text, "call"):
		if len(line.Tokens) != 2 {
			runtime.failExecutionLocked(execution, line, "call requires one function name")
			return
		}
		name := runtime.expandTokenLocked(line.Tokens[1], execution)
		index, ok := runtime.functions[name]
		if !ok {
			runtime.failExecutionLocked(execution, line, fmt.Sprintf("function %q is not defined", name))
			return
		}
		if len(execution.frames) >= legacyMacroMaxCallDepth {
			runtime.failExecutionLocked(execution, line, "maximum function call depth exceeded")
			return
		}
		execution.frames = append(execution.frames, legacyMacroCallFrame{declaration: index, elseIfLine: -1})
	case strings.EqualFold(first.Text, "message"):
		message := make([]string, 0, len(line.Tokens)-1)
		for _, token := range line.Tokens[1:] {
			message = append(message, runtime.expandTokenLocked(token, execution))
		}
		runtime.messageLocked(strings.Join(message, " "))
	case strings.EqualFold(first.Text, "move"):
		move, err := runtime.moveLocked(line, execution)
		if err != nil {
			runtime.failExecutionLocked(execution, line, err.Error())
			return
		}
		if runtime.hooks.Move != nil {
			runtime.hooks.Move(move)
		}
	default:
		runtime.appendTextLocked(execution, line.Tokens)
	}
}

func (runtime *legacyMacroRuntime) moveLocked(line legacyMacroLine, execution *legacyMacroExecution) (legacyMacroMove, error) {
	switch len(line.Tokens) {
	case 2:
		if strings.EqualFold(runtime.expandTokenLocked(line.Tokens[1], execution), "stop") {
			return legacyMacroMove{Direction: legacyMacroMoveStop}, nil
		}
	case 3:
		speed := runtime.expandTokenLocked(line.Tokens[1], execution)
		run := false
		switch {
		case strings.EqualFold(speed, "walk"):
		case strings.EqualFold(speed, "run"):
			run = true
		default:
			return legacyMacroMove{}, fmt.Errorf("move speed %q is not walk or run", speed)
		}
		directionName := runtime.expandTokenLocked(line.Tokens[2], execution)
		direction, ok := legacyMacroMoveDirectionByName(directionName)
		if !ok {
			return legacyMacroMove{}, fmt.Errorf("move direction %q is not recognized", directionName)
		}
		return legacyMacroMove{Direction: direction, Run: run}, nil
	}
	return legacyMacroMove{}, fmt.Errorf("move requires stop, or walk/run and a direction")
}

func legacyMacroMoveDirectionByName(name string) (legacyMacroMoveDirection, bool) {
	switch strings.ToLower(name) {
	case "stop":
		return legacyMacroMoveStop, true
	case "e", "east":
		return legacyMacroMoveEast, true
	case "ne", "northeast":
		return legacyMacroMoveNorthEast, true
	case "n", "north":
		return legacyMacroMoveNorth, true
	case "nw", "northwest":
		return legacyMacroMoveNorthWest, true
	case "w", "west":
		return legacyMacroMoveWest, true
	case "sw", "southwest":
		return legacyMacroMoveSouthWest, true
	case "s", "south":
		return legacyMacroMoveSouth, true
	case "se", "southeast":
		return legacyMacroMoveSouthEast, true
	default:
		return legacyMacroMoveStop, false
	}
}

func (runtime *legacyMacroRuntime) executeIfLocked(execution *legacyMacroExecution, currentFrame int, line legacyMacroLine, conditionStart int) {
	if len(line.Tokens) != conditionStart+3 {
		runtime.failExecutionLocked(execution, line, "if requires a value, comparison, and value")
		return
	}
	left := runtime.expandTokenLocked(line.Tokens[conditionStart], execution)
	comparison := runtime.expandTokenLocked(line.Tokens[conditionStart+1], execution)
	right := runtime.expandTokenLocked(line.Tokens[conditionStart+2], execution)
	passed, err := legacyMacroCompare(left, comparison, right)
	if err != nil {
		runtime.failExecutionLocked(execution, line, err.Error())
		return
	}
	if passed {
		return
	}

	current := &execution.frames[currentFrame]
	body := runtime.program.Macros[current.declaration].Body
	target, control, ok := legacyMacroFindControl(body, current.nextLine,
		legacyMacroControlElseIf, legacyMacroControlElse, legacyMacroControlEndIf)
	if !ok {
		runtime.failExecutionLocked(execution, line, "if has no matching end if")
		return
	}
	switch control {
	case legacyMacroControlElseIf:
		current.nextLine = target
		current.elseIfLine = target
	case legacyMacroControlElse:
		current.nextLine = target + 1
		current.elseIfLine = -1
	case legacyMacroControlEndIf:
		current.nextLine = target + 1
		current.elseIfLine = -1
	}
}

func (runtime *legacyMacroRuntime) skipToEndIfLocked(execution *legacyMacroExecution, currentFrame int, line legacyMacroLine) {
	current := &execution.frames[currentFrame]
	body := runtime.program.Macros[current.declaration].Body
	target, _, ok := legacyMacroFindControl(body, current.nextLine, legacyMacroControlEndIf)
	if !ok {
		runtime.failExecutionLocked(execution, line, "else has no matching end if")
		return
	}
	current.nextLine = target + 1
	current.elseIfLine = -1
}

func (runtime *legacyMacroRuntime) executeRandomLocked(execution *legacyMacroExecution, currentFrame, lineIndex int, line legacyMacroLine) {
	noRepeat := false
	switch len(line.Tokens) {
	case 1:
	case 2:
		if !strings.EqualFold(runtime.expandTokenLocked(line.Tokens[1], execution), "no-repeat") {
			runtime.failExecutionLocked(execution, line, "random only accepts the no-repeat option")
			return
		}
		noRepeat = true
	default:
		runtime.failExecutionLocked(execution, line, "random only accepts the no-repeat option")
		return
	}

	current := &execution.frames[currentFrame]
	body := runtime.program.Macros[current.declaration].Body
	branches, _, ok := legacyMacroRandomBranches(body, current.nextLine)
	if !ok {
		runtime.failExecutionLocked(execution, line, "random has no matching end random")
		return
	}
	key := legacyMacroInstruction{declaration: current.declaration, line: lineIndex}
	choice := runtime.randomChoiceLocked(key, len(branches), noRepeat)
	current.nextLine = branches[choice]
	current.elseIfLine = -1
}

func (runtime *legacyMacroRuntime) skipToEndRandomLocked(execution *legacyMacroExecution, currentFrame int, line legacyMacroLine) {
	current := &execution.frames[currentFrame]
	body := runtime.program.Macros[current.declaration].Body
	target, _, ok := legacyMacroFindControl(body, current.nextLine, legacyMacroControlEndRandom)
	if !ok {
		runtime.failExecutionLocked(execution, line, "or has no matching end random")
		return
	}
	current.nextLine = target + 1
	current.elseIfLine = -1
}

func (runtime *legacyMacroRuntime) executeGotoLocked(execution *legacyMacroExecution, currentFrame int, line legacyMacroLine) {
	if len(line.Tokens) != 2 {
		runtime.failExecutionLocked(execution, line, "goto requires one label")
		return
	}
	name := runtime.expandTokenLocked(line.Tokens[1], execution)
	current := &execution.frames[currentFrame]
	body := runtime.program.Macros[current.declaration].Body
	target, ok := legacyMacroFindLabel(body, name)
	if !ok {
		runtime.failExecutionLocked(execution, line, fmt.Sprintf("label %q is not defined in this macro", name))
		return
	}
	current.nextLine = target
	current.elseIfLine = -1
}

func (runtime *legacyMacroRuntime) randomChoiceLocked(key legacyMacroInstruction, choices int, noRepeat bool) int {
	if choices <= 1 {
		if noRepeat {
			runtime.randoms[key] = 0
		}
		return 0
	}

	lastChoice := -1
	if noRepeat {
		if previous, ok := runtime.randoms[key]; ok {
			lastChoice = previous
		}
	}
	choice := runtime.randomIntLocked(choices)
	if noRepeat && lastChoice >= 0 {
		for attempts := 0; choice == lastChoice && attempts < 16; attempts++ {
			choice = runtime.randomIntLocked(choices)
		}
		if choice == lastChoice {
			choice = (lastChoice + 1) % choices
		}
	}
	if noRepeat {
		runtime.randoms[key] = choice
	}
	return choice
}

func (runtime *legacyMacroRuntime) randomIntLocked(limit int) int {
	if limit <= 1 {
		return 0
	}
	if runtime.hooks.RandomInt == nil {
		return rand.Intn(limit)
	}
	value := runtime.hooks.RandomInt(limit)
	value %= limit
	if value < 0 {
		value += limit
	}
	return value
}

func legacyMacroCompare(left, comparison, right string) (bool, error) {
	leftNumber, leftErr := strconv.Atoi(left)
	rightNumber, rightErr := strconv.Atoi(right)
	if leftErr == nil && rightErr == nil {
		switch comparison {
		case ">":
			return leftNumber > rightNumber, nil
		case "<":
			return leftNumber < rightNumber, nil
		case ">=":
			return leftNumber >= rightNumber, nil
		case "<=":
			return leftNumber <= rightNumber, nil
		case "==":
			return leftNumber == rightNumber, nil
		case "!=":
			return leftNumber != rightNumber, nil
		default:
			return false, fmt.Errorf("comparison %q is not recognized", comparison)
		}
	}

	switch comparison {
	case ">":
		return strings.Contains(right, left) && !strings.EqualFold(left, right), nil
	case "<":
		return strings.Contains(left, right) && !strings.EqualFold(left, right), nil
	case ">=":
		return strings.Contains(left, right), nil
	case "<=":
		return strings.Contains(right, left), nil
	case "==":
		return strings.EqualFold(left, right), nil
	case "!=":
		return !strings.EqualFold(left, right), nil
	default:
		return false, fmt.Errorf("comparison %q is not recognized", comparison)
	}
}

func legacyMacroLineControl(line legacyMacroLine) legacyMacroControl {
	if len(line.Tokens) == 0 || line.Tokens[0].Quote != 0 {
		return legacyMacroControlNone
	}
	first := line.Tokens[0].Text
	switch {
	case strings.EqualFold(first, "if"):
		return legacyMacroControlIf
	case strings.EqualFold(first, "else"):
		if len(line.Tokens) > 1 && line.Tokens[1].Quote == 0 &&
			strings.EqualFold(line.Tokens[1].Text, "if") {
			return legacyMacroControlElseIf
		}
		return legacyMacroControlElse
	case strings.EqualFold(first, "end"):
		if len(line.Tokens) > 1 && line.Tokens[1].Quote == 0 {
			switch {
			case strings.EqualFold(line.Tokens[1].Text, "if"):
				return legacyMacroControlEndIf
			case strings.EqualFold(line.Tokens[1].Text, "random"):
				return legacyMacroControlEndRandom
			}
		}
		return legacyMacroControlEnd
	case strings.EqualFold(first, "random"):
		return legacyMacroControlRandom
	case strings.EqualFold(first, "or"):
		return legacyMacroControlOr
	case strings.EqualFold(first, "label"):
		return legacyMacroControlLabel
	case strings.EqualFold(first, "goto"):
		return legacyMacroControlGoto
	}
	return legacyMacroControlNone
}

func legacyMacroFindControl(body []legacyMacroLine, start int, wanted ...legacyMacroControl) (int, legacyMacroControl, bool) {
	depth := 0
	for index := start; index < len(body); index++ {
		control := legacyMacroLineControl(body[index])
		if depth == 0 {
			for _, candidate := range wanted {
				if control == candidate {
					return index, control, true
				}
			}
		}
		switch control {
		case legacyMacroControlIf, legacyMacroControlRandom:
			depth++
		case legacyMacroControlEndIf, legacyMacroControlEndRandom:
			if depth > 0 {
				depth--
			}
		}
	}
	return 0, legacyMacroControlNone, false
}

func legacyMacroRandomBranches(body []legacyMacroLine, start int) ([]int, int, bool) {
	branches := []int{start}
	depth := 0
	for index := start; index < len(body); index++ {
		control := legacyMacroLineControl(body[index])
		if depth == 0 {
			switch control {
			case legacyMacroControlOr:
				branches = append(branches, index+1)
				continue
			case legacyMacroControlEndRandom:
				return branches, index, true
			}
		}
		switch control {
		case legacyMacroControlIf, legacyMacroControlRandom:
			depth++
		case legacyMacroControlEndIf, legacyMacroControlEndRandom:
			if depth > 0 {
				depth--
			}
		}
	}
	return nil, 0, false
}

func legacyMacroFindLabel(body []legacyMacroLine, name string) (int, bool) {
	for index, line := range body {
		if legacyMacroLineControl(line) != legacyMacroControlLabel || len(line.Tokens) < 2 ||
			line.Tokens[1].Quote != 0 {
			continue
		}
		if line.Tokens[1].Text == name {
			return index, true
		}
	}
	return 0, false
}

func (runtime *legacyMacroRuntime) appendTextLocked(execution *legacyMacroExecution, tokens []legacyMacroToken) {
	for _, token := range tokens {
		execution.buffer += runtime.expandTokenLocked(token, execution)
	}
}

func (runtime *legacyMacroRuntime) setVariableLocked(line legacyMacroLine, execution *legacyMacroExecution, global bool) error {
	if len(line.Tokens) != 3 && len(line.Tokens) != 4 {
		return fmt.Errorf("set requires a variable and value, or a variable, operation, and value")
	}
	name := legacyMacroWritableVariableName(line.Tokens[1].Text)
	if name == "" {
		return fmt.Errorf("set variable name cannot be empty")
	}

	target := runtime.globals
	if !global && execution != nil {
		target = execution.variables
	}
	if len(line.Tokens) == 3 {
		target[name] = runtime.expandTokenLocked(line.Tokens[2], execution)
		return nil
	}

	value, ok := runtime.variableLocked(name, execution)
	if !ok {
		return fmt.Errorf("set variable %q is not defined", name)
	}
	operation := runtime.expandTokenLocked(line.Tokens[2], execution)
	other := runtime.expandTokenLocked(line.Tokens[3], execution)
	result, err := legacyMacroOperate(value, operation, other)
	if err != nil {
		return err
	}
	target[name] = result
	return nil
}

func legacyMacroOperate(left, operation, right string) (string, error) {
	leftNumber, leftErr := strconv.Atoi(left)
	rightNumber, rightErr := strconv.Atoi(right)
	leftIsNumber := leftErr == nil
	rightIsNumber := rightErr == nil

	switch operation {
	case "+":
		switch {
		case !leftIsNumber && !rightIsNumber:
			return left + right, nil
		case !leftIsNumber:
			return left + strconv.Itoa(rightNumber), nil
		case !rightIsNumber:
			return "", fmt.Errorf("cannot add a string to a number")
		default:
			return strconv.Itoa(leftNumber + rightNumber), nil
		}
	case "-":
		if !leftIsNumber || !rightIsNumber {
			return "", fmt.Errorf("subtraction requires numeric values")
		}
		return strconv.Itoa(leftNumber - rightNumber), nil
	case "*":
		if !leftIsNumber || !rightIsNumber {
			return "", fmt.Errorf("multiplication requires numeric values")
		}
		return strconv.Itoa(leftNumber * rightNumber), nil
	case "/":
		if !leftIsNumber || !rightIsNumber {
			return "", fmt.Errorf("division requires numeric values")
		}
		if rightNumber == 0 {
			return "", fmt.Errorf("division by zero")
		}
		return strconv.Itoa(leftNumber / rightNumber), nil
	case "%":
		if !leftIsNumber || !rightIsNumber {
			return "", fmt.Errorf("modulo requires numeric values")
		}
		if rightNumber == 0 {
			return "", fmt.Errorf("modulo by zero")
		}
		return strconv.Itoa(leftNumber % rightNumber), nil
	default:
		return "", fmt.Errorf("set operation %q is not recognized", operation)
	}
}

func (runtime *legacyMacroRuntime) expandTokenLocked(token legacyMacroToken, execution *legacyMacroExecution) string {
	if token.Quote != 0 {
		return token.Text
	}
	if value, ok := runtime.variableLocked(token.Text, execution); ok {
		return value
	}
	return token.Text
}

func (runtime *legacyMacroRuntime) variableLocked(name string, execution *legacyMacroExecution) (string, bool) {
	name = legacyMacroReadVariableName(name)
	if base, trailers, ok := legacyMacroSplitTextTrailers(name); ok {
		value, ok := runtime.variableBaseLocked(base, execution)
		if !ok {
			return "", false
		}
		return legacyMacroApplyTextTrailers(value, trailers), true
	}
	return runtime.variableBaseLocked(name, execution)
}

func (runtime *legacyMacroRuntime) variableBaseLocked(name string, execution *legacyMacroExecution) (string, bool) {
	if strings.EqualFold(name, "@random") {
		return strconv.Itoa(runtime.randomIntLocked(10000)), true
	}
	if runtime.hooks.ResolveState != nil {
		if value, ok := runtime.hooks.ResolveState(name); ok {
			return value, true
		}
	} else if value, ok := legacyMacroCurrentGameVariable(name); ok {
		return value, true
	}
	if execution != nil {
		if value, ok := execution.variables[name]; ok {
			return value, true
		}
	}
	value, ok := runtime.globals[name]
	return value, ok
}

func (runtime *legacyMacroRuntime) sendTextLocked(execution *legacyMacroExecution, text string) {
	runtime.sent = append(runtime.sent, text)
	if runtime.hooks.SendText != nil {
		runtime.hooks.SendText(text)
	}
	if value, ok := runtime.variableLocked("@env.echo", execution); ok && strings.EqualFold(value, "true") && runtime.hooks.SetText != nil {
		runtime.hooks.SetText(text)
	}
}

func (runtime *legacyMacroRuntime) messageLocked(message string) {
	runtime.messages = append(runtime.messages, message)
	if runtime.hooks.Message != nil {
		runtime.hooks.Message(message)
	}
}

func (runtime *legacyMacroRuntime) completeExecutionLocked(execution *legacyMacroExecution) {
	runtime.finishExecutionOutputLocked(execution)
	execution.complete = true
	if runtime.hooks.Complete != nil {
		runtime.hooks.Complete(execution)
	}
}

func (runtime *legacyMacroRuntime) failExecutionLocked(execution *legacyMacroExecution, line legacyMacroLine, message string) {
	runtime.recordDiagnosticLocked(line, message)
	execution.diagnostic = &runtime.diagnostics[len(runtime.diagnostics)-1]
	runtime.finishExecutionOutputLocked(execution)
	execution.complete = true
	if runtime.hooks.Complete != nil {
		runtime.hooks.Complete(execution)
	}
}

func (runtime *legacyMacroRuntime) finishExecutionOutputLocked(execution *legacyMacroExecution) {
	execution.result = execution.buffer
	if execution.captureOutput || execution.buffer == "" {
		return
	}
	switch execution.kind {
	case legacyMacroExpression:
		if runtime.hooks.SetText != nil {
			runtime.hooks.SetText(execution.buffer)
		}
	default:
		if runtime.hooks.InsertText != nil {
			runtime.hooks.InsertText(execution.buffer)
		}
	}
}

func (runtime *legacyMacroRuntime) recordDiagnosticLocked(line legacyMacroLine, message string) {
	location := legacyMacroLocation{Path: line.Source.Path, Line: line.Number, Column: 1}
	if len(line.Tokens) > 0 {
		location = tokenLocation(line, line.Tokens[0])
	}
	runtime.diagnostics = append(runtime.diagnostics, legacyMacroDiagnostic{
		Location: location,
		Message:  message,
	})
}

func (runtime *legacyMacroRuntime) globalsSnapshot() map[string]string {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	values := make(map[string]string, len(runtime.globals))
	for name, value := range runtime.globals {
		values[name] = value
	}
	return values
}

func (runtime *legacyMacroRuntime) sentSnapshot() []string {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return append([]string(nil), runtime.sent...)
}

func (runtime *legacyMacroRuntime) messagesSnapshot() []string {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return append([]string(nil), runtime.messages...)
}

func (runtime *legacyMacroRuntime) diagnosticsSnapshot() []legacyMacroDiagnostic {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return append([]legacyMacroDiagnostic(nil), runtime.diagnostics...)
}

func advanceLegacyMacros(frame int64) {
	legacyMacrosMu.RLock()
	runtime := legacyMacrosRuntime
	legacyMacrosMu.RUnlock()
	if runtime != nil {
		runtime.advance(frame)
	}
}
