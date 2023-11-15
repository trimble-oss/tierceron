package utils

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"sort"
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

func LineByLineDiff(stringA *string, stringB *string, patchData bool, colorSkip bool) string {
	//Colors used for output
	var Reset = "\033[0m"
	var Red = "\033[31m"
	var Green = "\033[32m"
	var Cyan = "\033[36m"
	var result string

	if IsWindows() {
		Reset = "\x1b[0m"
		Red = "\x1b[31m"
		Green = "\x1b[32m"
		Cyan = "\x1b[36m"
	} else if colorSkip {
		Reset = ""
		Red = ""
		Green = ""
		Cyan = ""
	}

	dmp := diffmatchpatch.New()
	var patchOutput string
	if patchData {
		var patchText string
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
	}

	//Diff Calculation
	diffTimeout := false
	timeOut := time.Now().Add(time.Minute * 1)
	if stringA == nil || stringB == nil {
		fmt.Println("A null string was found while diffing")
		return ""
	}
	diffs := dmp.DiffBisect(*stringA, *stringB, timeOut)
	diffs = dmp.DiffCleanupSemantic(diffs)

	if time.Now().After(timeOut) {
		diffTimeout = true
		diffs = diffs[:0]
	}

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
	if len(strings.TrimSpace(result)) == 0 && patchData {
		if diffTimeout {
			if IsWindows() {
				return "@@ Diff Timed Out @@"
			}
			return Cyan + "@@ Diff Timed Out @@" + Reset
		}

		if IsWindows() {
			return "@@ No Differences @@"
		}
		return Cyan + "@@ No Differences @@" + Reset
	} else {
		if patchOutput != "" {
			result = patchOutput + result
		}
		result = strings.TrimSuffix(result, "\n")
	}

	if IsWindows() {
		result = strings.ReplaceAll(result, Reset, "")
		result = strings.ReplaceAll(result, Green, "")
		result = strings.ReplaceAll(result, Cyan, "")
		result = strings.ReplaceAll(result, Red, "")
	}

	return result
}

func VersionHelper(versionData map[string]interface{}, templateOrValues bool, valuePath string, first bool) {
	Reset := "\033[0m"
	Cyan := "\033[36m"
	Red := "\033[31m"
	if IsWindows() {
		Reset = ""
		Cyan = ""
		Red = ""
	}

	if versionData == nil {
		fmt.Println("No version data found for this environment")
		return
	}

	//template == true
	if templateOrValues {
		for _, versionMap := range versionData {
			for _, versionMetadata := range versionMap.(map[string]interface{}) {
				for field, data := range versionMetadata.(map[string]interface{}) {
					if field == "destroyed" && !data.(bool) {
						goto printOutput1
					}
				}
			}
		}
		return

	printOutput1:
		for filename, versionMap := range versionData {
			fmt.Println(Cyan + "======================================================================================")
			fmt.Println(filename)
			fmt.Println("======================================================================================" + Reset)
			keys := make([]int, 0, len(versionMap.(map[string]interface{})))
			for versionNumber := range versionMap.(map[string]interface{}) {
				versionNo, err := strconv.Atoi(versionNumber)
				if err != nil {
					fmt.Println()
				}
				keys = append(keys, versionNo)
			}
			sort.Ints(keys)
			for i, key := range keys {
				versionNumber := fmt.Sprint(key)
				versionMetadata := versionMap.(map[string]interface{})[fmt.Sprint(key)]
				fmt.Println("Version " + string(versionNumber) + " Metadata:")

				fields := make([]string, 0, len(versionMetadata.(map[string]interface{})))
				for field := range versionMetadata.(map[string]interface{}) {
					fields = append(fields, field)
				}
				sort.Strings(fields)
				for _, field := range fields {
					fmt.Printf(field + ": ")
					fmt.Println(versionMetadata.(map[string]interface{})[field])
				}
				if i != len(keys)-1 {
					fmt.Println(Red + "-------------------------------------------------------------------------------" + Reset)
				}
			}
		}
		fmt.Println(Cyan + "======================================================================================" + Reset)
	} else {
		for _, versionMetadata := range versionData {
			for field, data := range versionMetadata.(map[string]interface{}) {
				if field == "destroyed" && !data.(bool) {
					goto printOutput
				}
			}
		}
		return

	printOutput:
		if len(valuePath) > 0 {
			if first {
				fmt.Println(Cyan + "======================================================================================" + Reset)
			}
			fmt.Println(valuePath)
		}

		fmt.Println(Cyan + "======================================================================================" + Reset)

		keys := make([]int, 0, len(versionData))
		for versionNumber := range versionData {
			versionNo, _ := strconv.ParseInt(versionNumber, 10, 64)
			keys = append(keys, int(versionNo))
		}
		sort.Ints(keys)
		for _, key := range keys {
			versionNumber := key
			versionMetadata := versionData[fmt.Sprint(key)]
			fields := make([]string, 0)
			fieldData := make(map[string]interface{}, 0)
			for field, data := range versionMetadata.(map[string]interface{}) {
				fields = append(fields, field)
				fieldData[field] = data
			}
			sort.Strings(fields)
			fmt.Println("Version " + fmt.Sprint(versionNumber) + " Metadata:")
			for _, field := range fields {
				fmt.Printf(field + ": ")
				fmt.Println(fieldData[field])
			}
			if keys[len(keys)-1] != versionNumber {
				fmt.Println(Red + "-------------------------------------------------------------------------------" + Reset)
			}
		}
		fmt.Println(Cyan + "======================================================================================" + Reset)
	}
}

func RemoveDuplicateValues(intSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}

	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func DiffHelper(configCtx *ConfigContext, config bool) {
	fileIndex := 0
	keys := []string{}
	configCtx.Mutex.Lock()
	if len(configCtx.ResultMap) == 0 {
		fmt.Println("Couldn't find any data to diff")
		return
	}

	var baseEnv []string
	diffEnvFound := false
	if len(configCtx.EnvSlice) > 0 {
		baseEnv = SplitEnv(configCtx.EnvSlice[0])
	}
	//Sort Diff Slice if env are the same
	for i, env := range configCtx.EnvSlice { //Arranges keys for ordered output
		var base []string = SplitEnv(env)

		if base[1] == "0" { //Special case for latest, so sort adds latest to the back of ordered slice
			base[1] = "_999999"
			configCtx.EnvSlice[i] = base[0] + base[1]
		}

		if len(base) > 0 && len(baseEnv) > 0 && baseEnv[0] != base[0] {
			diffEnvFound = true
		}
	}

	if !diffEnvFound {
		sort.Strings(configCtx.EnvSlice)
	}

	for i, env := range configCtx.EnvSlice { //Changes latest back - special case
		var base []string = SplitEnv(env)
		if base[1] == "999999" {
			base[1] = "_0"
			configCtx.EnvSlice[i] = base[0] + base[1]
		}
	}

	fileList := make([]string, configCtx.DiffFileCount)
	configCtx.Mutex.Unlock()

	sleepCount := 0
	if len(configCtx.ResultMap) != int(configCtx.DiffFileCount) {
		for {
			time.Sleep(time.Second)
			sleepCount++
			if sleepCount >= 20 {
				fmt.Println("Timeout: Attempted to wait for remaining configs to come in. Attempting incomplete diff.")
				break
			} else if len(configCtx.ResultMap) == int(configCtx.DiffFileCount)*configCtx.EnvLength {
				break
			}
		}
	}

	if config {
		//Make fileList
		for key := range configCtx.ResultMap {
			found := false
			keySplit := strings.Split(key, "||")

			for _, fileName := range fileList {
				if fileName == keySplit[1] {
					found = true
				}
			}

			if !found && len(fileList) > 0 && fileIndex < len(fileList) {
				fileList[fileIndex] = keySplit[1]
				fileIndex++
			}
		}
	} else {
		for _, env := range configCtx.EnvSlice { //Arranges keys for ordered output
			keys = append(keys, env+"||"+env+"_seed.yml")
		}
		if len(fileList) > 0 {
			fileList[0] = "placeHolder"
		} else {
			fileList = append(fileList, "placeHolder")
		}
	}

	//Diff resultMap using fileList
	for _, fileName := range fileList {
		if config {
			//Arranges keys for ordered output
			for _, env := range configCtx.EnvSlice {
				keys = append(keys, env+"||"+fileName)
			}
			if configCtx.FileSysIndex == len(configCtx.EnvSlice) {
				keys = append(keys, "filesys||"+fileName)
			}
		}

		Reset := "\033[0m"
		Red := "\033[31m"
		Green := "\033[32m"
		Yellow := "\033[0;33m"

		if IsWindows() {
			Reset = ""
			Red = ""
			Green = ""
			Yellow = ""
		}

		keyA := keys[0]
		keyB := keys[1]
		keySplitA := strings.Split(keyA, "||")
		keySplitB := strings.Split(keyB, "||")
		configCtx.Mutex.Lock()

		sortedKeyA := keyA
		sortedKeyB := keyB
		if _, ok := configCtx.ResultMap[sortedKeyA]; !ok {
			sortedKeyA = "||" + keySplitA[1]
		}
		if _, ok := configCtx.ResultMap[sortedKeyB]; !ok {
			sortedKeyB = "||" + keySplitB[1]
		}

		envFileKeyA := configCtx.ResultMap[sortedKeyA]
		envFileKeyB := configCtx.ResultMap[sortedKeyB]
		configCtx.Mutex.Unlock()

		latestVersionACheck := strings.Split(keySplitA[0], "_")
		if len(latestVersionACheck) > 1 && latestVersionACheck[1] == "0" {
			keySplitA[0] = strings.ReplaceAll(keySplitA[0], "0", "latest")
		}
		latestVersionBCheck := strings.Split(keySplitB[0], "_")
		if len(latestVersionBCheck) > 1 && latestVersionBCheck[1] == "0" {
			keySplitB[0] = strings.ReplaceAll(keySplitB[0], "0", "latest")
		}

		if strings.Count(keySplitA[1], "_") == 2 {
			fileSplit := strings.Split(keySplitA[1], "_")
			keySplitA[1] = fileSplit[0] + "_" + fileSplit[len(fileSplit)-1]
		}

		if strings.Count(keySplitB[1], "_") == 2 {
			fileSplit := strings.Split(keySplitB[1], "_")
			keySplitB[1] = fileSplit[0] + "_" + fileSplit[len(fileSplit)-1]
		}
		switch configCtx.EnvLength {
		case 4:
			keyC := keys[2]
			keyD := keys[3]
			keySplitC := strings.Split(keyC, "||")
			keySplitD := strings.Split(keyD, "||")
			configCtx.Mutex.Lock()
			envFileKeyC := configCtx.ResultMap[keyC]
			envFileKeyD := configCtx.ResultMap[keyD]
			configCtx.Mutex.Unlock()

			latestVersionCCheck := strings.Split(keySplitC[0], "_")
			if len(latestVersionCCheck) > 1 && latestVersionCCheck[1] == "0" {
				keySplitC[0] = strings.ReplaceAll(keySplitC[0], "0", "latest")
			}
			latestVersionDCheck := strings.Split(keySplitD[0], "_")
			if len(latestVersionDCheck) > 1 && latestVersionDCheck[1] == "0" {
				keySplitD[0] = strings.ReplaceAll(keySplitD[0], "0", "latest")
			}

			if strings.Count(keySplitC[1], "_") == 2 {
				fileSplit := strings.Split(keySplitC[1], "_")
				keySplitC[1] = fileSplit[0] + "_" + fileSplit[len(fileSplit)-1]
			}

			if strings.Count(keySplitD[1], "_") == 2 {
				fileSplit := strings.Split(keySplitD[1], "_")
				keySplitD[1] = fileSplit[0] + "_" + fileSplit[len(fileSplit)-1]
			}

			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyB, envFileKeyA, true, false))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyC, envFileKeyA, true, false))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyD, envFileKeyA, true, false))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyC, envFileKeyB, true, false))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyD, envFileKeyB, true, false))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitC[0] + Reset + Green + " +Env-" + keySplitD[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyD, envFileKeyC, true, false))
		case 3:
			keyC := keys[2]
			keySplitC := strings.Split(keyC, "||")
			configCtx.Mutex.Lock()
			envFileKeyC := configCtx.ResultMap[keyC]
			configCtx.Mutex.Unlock()

			latestVersionCCheck := strings.Split(keySplitC[0], "_")
			if len(latestVersionCCheck) > 1 && latestVersionCCheck[1] == "0" {
				keySplitC[0] = strings.ReplaceAll(keySplitC[0], "0", "latest")
			}

			if strings.Count(keySplitC[1], "_") == 2 {
				fileSplit := strings.Split(keySplitC[1], "_")
				keySplitC[1] = fileSplit[0] + "_" + fileSplit[len(fileSplit)-1]
			}

			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyB, envFileKeyA, true, false))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyC, envFileKeyA, true, false))
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitB[0] + Reset + Green + " +Env-" + keySplitC[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyC, envFileKeyB, true, false))
		default:
			fmt.Print("\n" + Yellow + keySplitA[1] + " (" + Reset + Red + "-Env-" + keySplitA[0] + Reset + Green + " +Env-" + keySplitB[0] + Reset + Yellow + ")" + Reset + "\n")
			fmt.Println(LineByLineDiff(envFileKeyB, envFileKeyA, true, false))
		}

		//Seperator
		if IsWindows() {
			fmt.Printf("======================================================================================\n")
		} else {
			fmt.Printf("\033[1;35m======================================================================================\033[0m\n")
		}
		keys = keys[:0] //Cleans keys for next file
	}
}
