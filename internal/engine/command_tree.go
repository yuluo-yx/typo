package engine

import (
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
	"github.com/yuluo-yx/typo/internal/utils"
)

type commandTreeTokenCandidate struct {
	token       string
	node        *itypes.CommandTreeNode
	distance    int
	similarity  float64
	lengthDelta int
}

func (e *Engine) tryCommandTreeFix(cmd string) itypes.FixResult {
	if e.commandTrees == nil {
		return itypes.FixResult{Fixed: false}
	}

	if result, parsed := e.tryCommandTreeFixWithShell(cmd); parsed {
		return result
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return itypes.FixResult{Fixed: false}
	}

	tree, rootToken, ok := e.matchCommandTreeRoot(parts[0])
	if !ok || tree == nil {
		return itypes.FixResult{Fixed: false}
	}

	rootChanged := rootToken != parts[0]
	matchedTokens, matchedChildren := e.matchCommandTreeTokens(parts[1:], tree.Node)
	if rootChanged && len(parts) > 1 && matchedChildren == 0 {
		return itypes.FixResult{Fixed: false}
	}

	changed := false
	if rootChanged {
		parts[0] = rootToken
		changed = true
	}
	for i, token := range matchedTokens {
		if token == parts[i+1] {
			continue
		}
		parts[i+1] = token
		changed = true
	}

	if !changed {
		return itypes.FixResult{Fixed: false}
	}

	return itypes.FixResult{
		Fixed:   true,
		Command: strings.Join(parts, " "),
		Source:  "tree",
	}
}

func (e *Engine) tryCommandTreeFixWithShell(cmd string) (itypes.FixResult, bool) {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return itypes.FixResult{Fixed: false}, false
	}

	for _, line := range lines {
		replacements := e.commandTreeReplacementsForLine(line)
		if len(replacements) == 0 {
			continue
		}

		return itypes.FixResult{
			Fixed:   true,
			Command: line.replaceWords(replacements...),
			Source:  "tree",
		}, true
	}

	return itypes.FixResult{Fixed: false}, true
}

func (e *Engine) commandTreeReplacementsForLine(line *shellCommandLine) []shellWordReplacement {
	if line == nil {
		return nil
	}

	tree, rootToken, ok := e.matchCommandTreeRoot(line.commandWord())
	if !ok || tree == nil {
		return nil
	}

	replacements := make([]shellWordReplacement, 0, 3)
	rootChanged := rootToken != line.commandWord()
	if rootToken != line.commandWord() {
		replacements = append(replacements, shellWordReplacement{
			index: line.commandIdx,
			value: rootToken,
		})
	}

	commandArgs := make([]string, 0, len(line.args)-line.commandIdx-1)
	for i := line.commandIdx + 1; i < len(line.args); i++ {
		commandArgs = append(commandArgs, line.args[i].Lit())
	}
	matchedTokens, matchedChildren := e.matchCommandTreeTokens(commandArgs, tree.Node)
	if rootChanged && line.commandIdx+1 < len(line.args) && matchedChildren == 0 {
		return nil
	}

	for i, token := range matchedTokens {
		if token == commandArgs[i] {
			continue
		}
		replacements = append(replacements, shellWordReplacement{
			index: line.commandIdx + 1 + i,
			value: token,
		})
	}

	if len(replacements) == 0 {
		return nil
	}

	return replacements
}

func (e *Engine) matchCommandTreeRoot(token string) (*itypes.CommandTree, string, bool) {
	if e.commandTrees == nil || token == "" {
		return nil, "", false
	}

	for _, tree := range e.commandTrees.Trees() {
		if tree != nil && tree.Root == token {
			return tree, token, true
		}
	}

	// Do not rewrite to the typo command tree when the first command is already executable.
	if e.hasKnownCommand(token) {
		return nil, "", false
	}

	best := commandTreeTokenCandidate{distance: 999, similarity: -1}
	var bestTree *itypes.CommandTree
	matchCfg := e.distanceMatchConfig()

	for _, tree := range e.commandTrees.Trees() {
		if tree == nil {
			continue
		}

		candidate, ok := newCommandTreeTokenCandidate(token, tree.Root, tree.Node, matchCfg, e.keyboard)
		if !ok {
			continue
		}
		if !hasBetterCommandTreeCandidate(candidate, best) {
			continue
		}

		best = candidate
		bestTree = tree
	}

	if bestTree == nil {
		return nil, "", false
	}

	return bestTree, best.token, true
}

func (e *Engine) findCommandTreeRootForArgs(token string, args []string) string {
	tree, replacement, ok := e.matchCommandTreeRoot(token)
	if !ok || replacement == token {
		return ""
	}
	if len(args) == 0 {
		return replacement
	}
	if tree == nil || tree.Node == nil {
		return ""
	}

	_, _, matched := e.matchCommandTreeChild(args[0], tree.Node)
	if !matched {
		return ""
	}

	return replacement
}

func (e *Engine) matchCommandTreeTokens(tokens []string, node *itypes.CommandTreeNode) ([]string, int) {
	if len(tokens) == 0 || node == nil {
		return nil, 0
	}

	matchedTokens := make([]string, 0, len(tokens))
	currentNode := node
	matchedChildren := 0

	for _, token := range tokens {
		if currentNode == nil || currentNode.StopAfterMatch {
			break
		}

		replacement, child, matched := e.matchCommandTreeChild(token, currentNode)
		if !matched {
			break
		}

		matchedTokens = append(matchedTokens, replacement)
		matchedChildren++
		currentNode = child
	}

	return matchedTokens, matchedChildren
}

func (e *Engine) matchCommandTreeChild(token string, node *itypes.CommandTreeNode) (string, *itypes.CommandTreeNode, bool) {
	if node == nil || token == "" {
		return "", nil, false
	}

	if child, ok := node.Child(token); ok {
		return token, child, true
	}

	childTokens := node.ChildTokens()
	if len(childTokens) == 0 {
		return "", nil, false
	}

	matchCfg := e.distanceMatchConfig()
	best := commandTreeTokenCandidate{distance: 999, similarity: -1}
	for _, childToken := range childTokens {
		child, _ := node.Child(childToken)
		candidate, ok := newCommandTreeTokenCandidate(token, childToken, child, matchCfg, e.keyboard)
		if !ok {
			continue
		}
		if hasBetterCommandTreeCandidate(candidate, best) {
			best = candidate
		}
	}

	if best.token == "" {
		return "", nil, false
	}

	return best.token, best.node, true
}

func newCommandTreeTokenCandidate(original, candidate string, node *itypes.CommandTreeNode, cfg distanceMatchConfig, keyboard KeyboardWeights) (commandTreeTokenCandidate, bool) {
	distance := Distance(original, candidate, keyboard)
	if !isGoodCommandTreeTokenMatch(original, candidate, distance, cfg, keyboard) {
		return commandTreeTokenCandidate{}, false
	}

	return commandTreeTokenCandidate{
		token:       candidate,
		node:        node,
		distance:    distance,
		similarity:  SimilarityFromDistance(len(original), len(candidate), distance),
		lengthDelta: utils.Abs(len([]rune(original)) - len([]rune(candidate))),
	}, true
}

func hasBetterCommandTreeCandidate(candidate, current commandTreeTokenCandidate) bool {
	if candidate.distance != current.distance {
		return candidate.distance < current.distance
	}
	if candidate.similarity != current.similarity {
		return candidate.similarity > current.similarity
	}
	if candidate.lengthDelta != current.lengthDelta {
		return candidate.lengthDelta < current.lengthDelta
	}
	return candidate.token < current.token
}

func isGoodCommandTreeTokenMatch(original, candidate string, distance int, cfg distanceMatchConfig, keyboard KeyboardWeights) bool {
	if isGoodDistanceMatch(original, candidate, distance, cfg) {
		return true
	}

	if distance > cfg.maxEditDistance {
		return false
	}

	if utils.IsSingleAdjacentTransposition(original, candidate) {
		return true
	}

	return isShortBoundaryPreservingMatch(original, candidate, distance)
}

func isShortBoundaryPreservingMatch(original, candidate string, distance int) bool {
	originalRunes := []rune(original)
	candidateRunes := []rune(candidate)
	if distance <= 0 || len(originalRunes) != len(candidateRunes) {
		return false
	}
	if len(originalRunes) < 3 || len(originalRunes) > 4 {
		return false
	}

	last := len(originalRunes) - 1
	return originalRunes[0] == candidateRunes[0] && originalRunes[last] == candidateRunes[last]
}
