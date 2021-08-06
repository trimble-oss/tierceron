package utils

import (
	"bytes"
	"log"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func GetStringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	s += len(start)
	e := strings.Index(str[s:], end)
	if e == -1 {
		return
	}
	return str[s : s+e]
}

func LineByLineDiff(stringA *string, stringB *string) string {
	//Colors used for output
	var Reset = "\033[0m"
	var Red = "\033[31m"
	var Green = "\033[32m"
	var Cyan = "\033[36m"
	var result string

	if runtime.GOOS == "windows" {
		Reset = "\x1b[0m"
		Red = "\x1b[31m"
		Green = "\x1b[32m"
		Cyan = "\x1b[36m"
	}

	dmp := diffmatchpatch.New()
	var patchText string
	var patchOutput string
	//Patch Calculation - Catches patch slice out of bounds
	func() {
		defer func() {
			if r := recover(); r != nil {
				patchText = ""
			}
		}()
		patches := dmp.PatchMake(*stringA, *stringB) //This throws out of index slice error rarely
		patchText = dmp.PatchToText(patches)
	}()

	if patchText != "" {
		//Converts escaped chars in patches
		unescapedPatchText, err2 := url.PathUnescape(patchText)
		if err2 != nil {
			log.Fatalf("Unable to decode percent-encoding: %v", err2)
		}

		parsedPatchText := strings.Split(unescapedPatchText, "\n")

		//Fixes char offset due to common preString
		for i, string := range parsedPatchText {
			if strings.Contains(string, "@@") {
				charOffset := string[strings.Index(parsedPatchText[i], "-")+1 : strings.Index(parsedPatchText[i], ",")]
				charOffsetInt, _ := strconv.Atoi(charOffset)
				charOffsetInt = charOffsetInt - 2 + len(parsedPatchText[i+1])
				parsedPatchText[i] = strings.Replace(string, charOffset, strconv.Itoa(charOffsetInt), 2)
			}
		}

		//Grabs only patch data from PatchMake
		onlyPatchedText := []string{}
		for _, stringLine := range parsedPatchText {
			if strings.Contains(stringLine, "@@") {
				onlyPatchedText = append(onlyPatchedText, stringLine)
			}
		}

		//Patch Data Output
		patchOutput = Cyan + strings.Join(onlyPatchedText, " ") + Reset + "\n"
	} else {
		patchOutput = Cyan + "@@ Patch Data Unavailable @@" + Reset + "\n"
	}

	//Diff Calculation
	timeOut := time.Date(9999, 1, 1, 12, 0, 0, 0, time.UTC)
	diffs := dmp.DiffBisect(*stringA, *stringB, timeOut)
	diffs = dmp.DiffCleanupSemantic(diffs)

	//Seperates diff into red and green lines
	var redBuffer bytes.Buffer
	var greenBuffer bytes.Buffer
	for _, diff := range diffs {
		text := diff.Text
		switch diff.Type {
		case diffmatchpatch.DiffDelete:
			_, _ = greenBuffer.WriteString(Green)
			_, _ = greenBuffer.WriteString(text)
			_, _ = greenBuffer.WriteString(Reset)
		case diffmatchpatch.DiffInsert:
			_, _ = redBuffer.WriteString(Red)
			_, _ = redBuffer.WriteString(text)
			_, _ = redBuffer.WriteString(Reset)
		case diffmatchpatch.DiffEqual:
			_, _ = redBuffer.WriteString(text)
			_, _ = greenBuffer.WriteString(text)
		}
	}

	greenLineSplit := strings.Split(greenBuffer.String(), "\n")
	redLineSplit := strings.Split(redBuffer.String(), "\n")

	//Adds + for each green line
	for greenIndex, greenLine := range greenLineSplit {
		if strings.Contains(greenLine, Green) {
			greenLineSplit[greenIndex] = "+" + greenLine
		}
	}

	//Adds - for each red line
	for redIndex, redLine := range redLineSplit {
		if strings.Contains(redLine, Red) {
			redLineSplit[redIndex] = "-" + redLine
		}
	}

	//Red vs Green length
	lengthDiff := 0
	sameLength := 0
	var redSwitch bool
	if len(redLineSplit) > len(greenLineSplit) {
		redSwitch = true
		lengthDiff = len(redLineSplit) - len(greenLineSplit)
		sameLength = len(greenLineSplit)
	} else { //Green > Red
		redSwitch = false
		lengthDiff = len(greenLineSplit) - len(redLineSplit)
		sameLength = len(redLineSplit)
	}

	//Prints line-by-line until shorter length
	currentIndex := 0
	for currentIndex != sameLength {
		redLine := redLineSplit[currentIndex]
		greenLine := greenLineSplit[currentIndex]
		if len(redLine) > 0 && redLine[0] == '-' {
			result += redLine + "\n"
		}
		if len(greenLine) > 0 && greenLine[0] == '+' {
			result += greenLine + "\n"
		}
		currentIndex++
	}

	//Prints rest of longer length
	for currentIndex != lengthDiff+sameLength {
		if redSwitch {
			redLine := redLineSplit[currentIndex]
			if len(redLine) > 0 && redLine[0] == '-' {
				result += redLine + "\n"
			}
		} else {
			greenLine := greenLineSplit[currentIndex]
			if len(greenLine) > 0 && greenLine[0] == '+' {
				result += greenLine + "\n"
			}
		}
		currentIndex++
	}

	//Colors first line "+" & "-"
	if len(result) > 0 && string(result[0]) == "+" {
		result = strings.Replace(result, "+", Green+"+"+Reset, 1)
	} else if len(result) > 0 && string(result[0]) == "-" {
		result = strings.Replace(result, "-", Red+"-"+Reset, 1)
	}

	//Colors all "+" & "-" using previous newline
	result = strings.ReplaceAll(result, "\n", Reset+"\n")
	result = strings.ReplaceAll(result, "\n+", "\n"+Green+"+"+Reset)
	result = strings.ReplaceAll(result, "\n-", "\n"+Red+"-"+Reset)

	//Diff vs no Diff output
	if len(strings.TrimSpace(result)) == 0 {
		if runtime.GOOS == "windows" {
			return "@@ No Differences @@"
		}
		return Cyan + "@@ No Differences @@" + Reset
	} else {
		result = patchOutput + result
		result = strings.TrimSuffix(result, "\n")
	}

	if runtime.GOOS == "windows" {
		result = strings.ReplaceAll(result, Reset, "")
		result = strings.ReplaceAll(result, Green, "")
		result = strings.ReplaceAll(result, Cyan, "")
		result = strings.ReplaceAll(result, Red, "")
	}

	return result
}
