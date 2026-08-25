//go:build script

package main

import (
	"strings"
	"time"

	"gt2"
)

// script metadata
const scriptName = "Ledger Actions"
const scriptID = "ledger"
const scriptAuthor = "Examples"
const scriptCategory = "Tools"
const scriptAPIVersion = 2

var fighters = []string{
	"Angilsa", "Aktur", "Atkia", "Atkus", "Balthus", "Bodrus", "Darkus",
	"Detha", "Evus", "Histia", "Knox", "Regia", "Rodnus", "Swengus",
	"Bangus", "Duvin", "Respin", "SplashOSul", "Farly", "Anemia",
	"Stedfustus", "Aneurus", "Erthron", "Forvyola", "Corsetta",
	"Toomeria", "ValaLoak",
}

var healers = []string{
	"AnAnFaure", "AnDeuxFaure", "AnTrixFaure", "AnQuartFaure",
	"AnSeptFaure", "Awaria", "Eva", "Faustus", "Higgrus", "Horus",
	"Proximus", "Radium", "Respia", "Sespus", "Sprite", "Spirtus",
}

var others = []string{
	"Asteshasha", "DentirLongtooth", "Mentus", "Skea", "Troilus",
	"Sartorio", "Vorharn", "LanaGaraka", "ParTroon", "Frrinakin",
	"BabelleLyrn", "Sporrin",
}

const pauseDuration = 3 * time.Second

func Init() {
	gt2.Command("ledgerfind", ledgerFind)
	gt2.Command("ledgerlanguage", ledgerLanguage)
}

func ledgerFind(args string) {
	gt2.Print("ledger: find trainers")
	gt2.Send("/equip trainingledger")
	fields := strings.Fields(args)
	// (trimmed debug output)
	playerName := gt2.Self().Name
	category := ""
	if len(fields) > 0 {
		category = strings.ToLower(fields[0])
	}
	if len(fields) > 1 {
		playerName = fields[1]
	}
	if category == "healer" || category == "all" {
		for _, h := range healers {
			gt2.Send("/use " + h + " /judge " + playerName)
			gt2.Wait(pauseDuration)
		}
	}
	if category == "fighter" || category == "all" {
		for _, f := range fighters {
			gt2.Send("/use " + f + " /judge " + playerName)
			gt2.Wait(pauseDuration)
		}
	}
	if category == "other" || category == "all" {
		for _, o := range others {
			gt2.Send("/use " + o + " /judge " + playerName)
			gt2.Wait(pauseDuration)
		}
	}
}

func ledgerLanguage(args string) {
	gt2.Print("ledger: judge language")
	gt2.Send("/equip trainingledger")
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return
	}
	playerName := fields[0]
	gt2.Send("/use babellelyrn /judge " + playerName)
	if len(fields) > 1 {
		gt2.Send("/use babellelyrn /judge " + fields[1])
	}
}
