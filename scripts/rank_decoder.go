//go:build plugin

package main

import "gt"

// Plugin metadata
var PluginName = "Rank Decoder"
var PluginAuthor = "Examples"
var PluginCategory = "Tools"

const PluginAPIVersion = 1

// rankMessages maps trainer phrases to their numerical rank ranges.
var rankMessages = map[string]string{
	"You have much to learn.":                           "0-9",
	"You feel you have much to learn.":                  "0-9",
	"It is good to see you.":                            "10-19",
	"You feel tolerably skilled.":                       "10-19",
	"Your persistence is paying off.":                   "20-29",
	"You are progressing well.":                         "30-39",
	"You are a good pupil of mine.":                     "40-49",
	"You are becoming proficient.":                      "40-49",
	"You are one of my better pupils.":                  "50-99",
	"You have learned much.":                            "50-99",
	"You keep me on my toes.":                           "100-149",
	"You have become skilled.":                          "100-149",
	"It is hard to find more to teach you.":             "150-199",
	"You have become very skilled.":                     "150-199",
	"Teaching you is a challenge.":                      "200-249",
	"Learning more is a challenge.":                     "200-249",
	"There is not much more I can teach you.":           "250-299",
	"You have attained great skill.":                    "250-299",
	"Teaching you has taught me much.":                  "300-349",
	"You are becoming an expert.":                       "300-349",
	"You have attained tremendous skill.":               "350-399",
	"We are nearly equals.":                             "400-449",
	"You are close to attaining mastery.":               "400-449",
	"You may be proud of your accomplishment.":          "450-499",
	"You are becoming a master of your art.":            "500-549",
	"Your dedication is commendable.":                   "550-599",
	"You show great devotion to your studies.":          "600-649",
	"You are a credit to our craft.":                    "650-699",
	"You are a credit to your craft.":                   "650-699",
	"Few indeed are your peers.":                        "700-749",
	"Your devotion to the craft is exemplary.":          "750-799",
	"Your devotion to your craft is exemplary.":         "750-799",
	"It is always good to greet a respected colleague.": "800-899",
	"Your expertise is unquestionable.":                 "800-899",
	"You are truly a grand master.":                     "900-999",
	"Let us search for more we might learn together.":   "1000-1249",
	"Few if any are your equal.":                        "1000-1249",
	"Your persistence is an example to us all.":         "1250-1499",
	"Your skill astounds me.":                           "1500-1749",
	"Your skill is astounding.":                         "1500-1749",
	"You have progressed further than most.":            "1750-1999",
	"You are nearly peerless.":                          "2000-2249",
	"You are a model of dedication.":                    "2250-2499",
	"You have achieved mastery.":                        "2500-2749",
	"You are enlightened.":                              "2750-2999",
	"Your command of our craft is inspiring.":           "3000-3249",
	"All commend your dedication to our craft.":         "3250-3499",
	"I marvel at your skill.":                           "3500-3749",
	"You walk where few have tread.":                    "3750-3999",
	"Few stones are unturned in your path.":             "4000-4249",
	"Your footsteps guide the dedicated.":               "4250-4499",
	"You chart a way through the unknown.":              "4500-4749",
	"Your path illuminates the wilderness.":             "4750-4999",
	"Your path is ablaze with glory.":                   "6000-",
	"You are enlightened beyond measure.":               "???",
	"There is nothing I can teach you.":                 "MAXED",
}

func Init() {
	gt.Chat("", rankDecodeChat)
}

func rankDecodeChat(msg string) {
	for phrase, rank := range rankMessages {
		if gt.Includes(msg, phrase) {
			gt.ShowNotification("Rank " + rank)
			break
		}
	}
}
