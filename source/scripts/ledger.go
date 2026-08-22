//go:build script

package main

import (
	"time"

	"gt"
)

// script metadata
const scriptName = "Ledger Actions"
const scriptAuthor = "Examples"
const scriptCategory = "Tools"
const scriptAPIVersion = 1

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
	gt.RegisterCommand("ledgerfind", ledgerFind)
	gt.RegisterCommand("ledgerlanguage", ledgerLanguage)
}

func ledgerFind(args string) {
	gt.Print("ledger: find trainers")
	gt.Run("/equip trainingledger")
	fields := gt.Words(args)
	// (trimmed debug output)
	playerName := gt.PlayerName()
	category := ""
	if len(fields) > 0 {
		category = gt.Lower(fields[0])
	}
	if len(fields) > 1 {
		playerName = fields[1]
	}
	if category == "healer" || category == "all" {
		for _, h := range healers {
			gt.Run("/use " + h + " /judge " + playerName)
			time.Sleep(pauseDuration)
		}
	}
	if category == "fighter" || category == "all" {
		for _, f := range fighters {
			gt.Run("/use " + f + " /judge " + playerName)
			time.Sleep(pauseDuration)
		}
	}
	if category == "other" || category == "all" {
		for _, o := range others {
			gt.Run("/use " + o + " /judge " + playerName)
			time.Sleep(pauseDuration)
		}
	}
}

func ledgerLanguage(args string) {
	gt.Print("ledger: judge language")
	gt.Run("/equip trainingledger")
	fields := gt.Words(args)
	if len(fields) == 0 {
		return
	}
	playerName := fields[0]
	gt.Run("/use babellelyrn /judge " + playerName)
	if len(fields) > 1 {
		gt.Run("/use babellelyrn /judge " + fields[1])
	}
}
