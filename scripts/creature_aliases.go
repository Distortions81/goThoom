//go:build plugin

package main

import "gt"

// Plugin metadata
const PluginName = "Creature Aliases"
const PluginAuthor = "Examples"
const PluginCategory = "Fun"
const PluginAPIVersion = 1

// Init registers a bunch of chat shortcuts so typing
// "abo" automatically expands to the full creature name.
func Init() {
	gt.AddShortcuts(map[string]string{
		"abo":   "Abominable Snow Yorilla",
		"asy":   "Abominable Snow Yorilla",
		"ag":    "Agronox",
		"az":    "Azurite Arachnoid",
		"ban":   "Banana Arachne",
		"bf":    "Barrens Feral",
		"bl":    "Black Locust",
		"bliz":  "Blizzard Greymyr",
		"blizz": "Blizzard Greymyr",
		"cc":    "cave cobra",
		"corc":  "Corrosive Corpse",
		"crim":  "Crimson Arachnoid",
		"kes":   "Crookbeak Kestrel",
		"dk":    "Dar'kin",
		"da":    "Darshak Assassin",
		"dd":    "Darshak Defender",
		"dg":    "Darshak Guardian",
		"dh":    "Darshak Harrier",
		"ds":    "Darshak Spirit",
		"dt":    "Delta Toad",
		"ec":    "eolith crawler",
		"es":    "Ethereal Slug",
		"et":    "elder tae su",
		"fk":    "Fledgling Kestrel",
		"flot":  "Flotsam Meshra",
		"fy":    "frost yorilla",
		"liz":   "Giant River Lizard",
		"gbc":   "Glacier Bear Cubr",
		"hare":  "Haremau",
		"hk":    "Haremau Kitten",
		"hhb":   "Hungry Hell Boar",
		"ba":    "infant ebb meshra",
		"iem":   "infant ebb meshra",
		"jade":  "Jade Arachnoid",
		"lh":    "Lowland Hawk",
		"lp":    "Lowland Panther",
		"mra":   "Mammoth Recluse Arachne",
		"msa":   "Mammoth Stone Arachne",
		"mf":    "Mange Feral",
		"mk":    "Mistral Kestrel",
		"ms":    "Mauling Spirit",
		"trmw":  "mountain wasp",
		"mala":  "Malachite Arachnoid",
		"mw":    "Mountain Wasp",
		"mb":    "Mountain Bison",
		"griz":  "Mountain Grizzly Bear",
		"mp":    "Mountain Panther",
		"nw":    "Nocturne Wendecka",
		"ob":    "Orga Berserk",
		"olive": "Olive Arachnoid",
		"pitch": "Pitch Arachnoid",
		"ry":    "Raja-Yorilla",
		"ra":    "Recluse Arachne",
		"sls":   "sao-la starfawn",
		"saz":   "Sazaja",
		"hunt":  "Shadowcat Huntress",
		"wq":    "Shadowcat Warqueen",
		"sb":    "Sky Bison",
		"slate": "Slate Arachnoid",
		"stag":  "Snowstag",
		"star":  "Starstag",
		"strip": "Striped Azurite Arachnoid",
		"ty":    "Tawny Yorilla",
		"tear":  "Tearer",
		"uty":   "Utsanna Tawny Yorilla",
		"va":    "Violène Arachne",
		"ws":    "Wild Stallion",
	})
}
