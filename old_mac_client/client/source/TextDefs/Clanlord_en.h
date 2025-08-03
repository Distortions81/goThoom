/*
**	Text definitions used by English version of Clan Lord 
** 
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**     https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

// support
#define LDQUO	"\322"		// left double curly quote
#define	RDQUO	"\323"		// right ditto


// Texts used in Main_cl.cp
#define TXTCL_WIN_TITLE TXTCL_APP_NAME " v%s"
#define TXTCL_CLIENT_WEB_HELP "Help on the Web"
#define TXTCL_SELECT_MOVIE_FILE "Please select a " TXTCL_APP_NAME " Movie file."
#define TXTCL_STARTING_MOVIE "* Starting movie \"%s\" *"
#define TXTCL_END_OF_MOVIE "* End of movie \"%s\" *"
#define TXTCL_MOVIE_NOT_READABLE "That movie could not be read."
#define TXTCL_MOVIE_TO_OLD "That movie is too old to be viewed with this version of " TXTCL_APP_NAME "."
#define TXTCL_MOVIE_TO_NEW "That movie is too new to be viewed with this version of " TXTCL_APP_NAME "."
#define TXTCL_END_OF_MOVIE_FILE "*** End of movie file. ***"
#define TXTCL_NO_LONGER_CONNECTED "*** We are no longer connected to the " TXTCL_APP_NAME " game server. ***"
#define TXTCL_AUTO_HIDE_NAMES "Auto-Hide Names"
#define TXTCL_RADIO_CLICKHOLD "Holding the mouse down will cause your character to move towards the mouse. The farther away it is the faster you will move."
#define TXTCL_RADIO_CLICKTOGGLES "Click and release will cause your character to move towards the mouse. The farther away it is the faster you will move. Click again to stop chasing the mouse."
#define TXTCL_MOVIE_ERR_ALREADY_RECORDING "A movie is already being recorded."
#define TXTCL_MOVIE_ERR_NOT_RECORDING_WHILE_PLAYING "Cannot record a movie while playing one."
#define TXTCL_MOVIE_ERR_NO_MOVIE_RECORDING "No movie is being recorded."

// Texts used in Comm_cl.cp
#define TXTCL_PREF_FILE_PROBLEM "There seems to be a problem with the file \"CL_Prefs\". Please quit and drag it into the Trash."
#define TXTCL_FAILED_INIT_COMM "Failed to initialize communications: %d."
#define TXTCL_SERVER_UNREACHABLE "The " TXTCL_APP_NAME " game server seems to be unavailable or unreachable at this time.\r\rPlease try to connect again later."
#define TXTCL_FAILED_FIND_SERVER "* Failed %d to find server at '%s'."
#define TXTCL_SERVER_NO_RESPONSE "The " TXTCL_APP_NAME " game server failed to respond."
#define TXTCL_UPDATE_FROM_WEB "Please visit the " TXTCL_APP_NAME " web site %s to update to the newest version."
#define TXTCL_DISCONNECTED "* Got disconnected from game host."
#define TXTCL_LOOKING_FOR_SERVER "Looking for " TXTCL_APP_NAME " Server..."

// Texts used in Commands_cl.cp
#define TXTCL_CMD_HELP_HOLD "Sets the movement mode to click and hold."
#define TXTCL_CMD_HELP_TOGGLE "Sets movement mode to click toggles."
#define TXTCL_CMD_HELP_CLANNING "\\PREF MESSAGE CLANNING <TRUE/FALSE> Will set whether clanning messages are shown."
#define TXTCL_CMD_HELP_FALLEN "\\PREF MESSAGE FALLEN <TRUE/FALSE> Will set whether fallen messages are shown."
#define TXTCL_CMD_HELP_MOVEMENT "\\PREF MOVEMENT <MODE> Will set your movement mode."
//#define TXTCL_CMD_HELP_LARGEWINDOW "\\PREF LARGEWINDOW <TRUE/FALSE> Will set your window size."
#define TXTCL_CMD_HELP_MESSAGE "\\PREF MESSAGE <TYPE> <TRUE/FALSE> Will set whether messages are shown."
#define TXTCL_CMD_HELP_BRIGHTCOLORS "\\PREF BRIGHTCOLORS <TRUE/FALSE> Will set whether to use bright colors for under names."
#define TXTCL_CMD_HELP_AUTOHIDE "\\PREF AUTOHIDE <TRUE/FALSE> Will set whether to auto-hide player names."
#define TXTCL_CMD_HELP_SHOWNAMES "\\PREF SHOWNAMES <TRUE/FALSE> Will set whether to show player names."
#define TXTCL_CMD_HELP_TIMESTAMPS "\\PREF TIMESTAMPS <TRUE/FALSE> Will set whether to show time stamps in the message log."
#define TXTCL_CMD_HELP_MAXNIGHTPERCENT "\\PREF MAXNIGHTPERCENT <PERCENT> Sets the maximum night percentage."
#define TXTCL_CMD_HELP_VOLUME "\\PREF VOLUME <0-100> Sets the sound volume."
#define TXTCL_CMD_HELP_BARDVOLUME "\\PREF BARDVOLUME <0-100> Sets the bard song volume."
#define TXTCL_CMD_HELP_NEWLOG "\\PREF NEWLOG <TRUE/FALSE> Will set whether to start a new text log file on every Join."
#define TXTCL_CMD_HELP_MOVIELOGS "\\PREF MOVIELOGS <TRUE/FALSE> Will set whether to save text logs when playing movies."
#define TXTCL_CMD_HELP_LABEL "\\LABEL <PLAYER> <LABEL> Labels a player as a friend with that label.  Possible labels are red, orange, green, blue, purple, and none.  \\FORGET will also remove a label."
#define TXTCL_CMD_HELP_BLOCK "\\BLOCK <PLAYER> Sets a player to be blocked. You will not hear anything they say."
#define TXTCL_CMD_HELP_FORGET "\\FORGET <PLAYER> Undoes a block, label, or ignore."
#define TXTCL_CMD_HELP_IGNORE  "\\IGNORE <PLAYER> Sets a player to be ignored. You will not hear anything they say, and they will be invisible."
#define TXTCL_CMD_HELP_MOVE "\\MOVE <DIRECTION> <SPEED> causes your character to move in direction at speed. Speed may be STOP, WALK, or RUN."
#define TXTCL_CMD_HELP_PREF "\\PREF <PREFERENCE> <VALUE> Sets a client preference."
#define TXTCL_CMD_HELP_RECORD "\\RECORD <ON/OFF> starts or stops recording a movie."
#define TXTCL_CMD_HELP_SELECT "\\SELECT <PLAYER> Selects the player. <PLAYER> may also be @first, @last, @prev, @next."
//#define TXTCL_CMD_HELP_EQUIP "\\EQUIP <ITEM NAME> Equips the named item."
//#define TXTCL_CMD_HELP_UNEQUIP "\\UNEQUIP <HAND> Empties the chosen hand."
#define TXTCL_CMD_HELP_SELECTITEM "\\SELECTITEM <ItemName> Selects the named item. Examples:\r \\SELECTITEM Purse\r \\SELECTITEM \"Shirt <#2>\""
#define TXTCL_CMD_HELP_WHOLABEL "\\WHOLABEL <LABEL> lists which people with that label are clanning.  With no <LABEL>, lists all labeled people clanning."
#define TXTCL_CMD_ERR_NONUMBER "'%s' is not a number."
#define TXTCL_CMD_ERR_NOTTRUEFALSE "'%s' is not a TRUE/FALSE value."
#define TXTCL_CMD_ERR_UNKNOWNLABEL "Unknown label '%s'"
#define TXTCL_CMD_ERR_NORELATIVESELECTIONS "* HandleSelectItemCommand: relative selections not yet implemented."
#define TXTCL_CMD_ERR_MOVIEALREADYRECORDING "* A movie is already being recorded."
#define TXTCL_CMD_ERR_NORECORDWHILEPLAYINGONE "* Cannot record a movie while playing one."
#define TXTCL_CMD_ERR_RECORDINGSTARTED "* Recording started."
#define TXTCL_CMD_ERR_RECORDINGSTOPPED "* Recording stopped."
#define TXTCL_CMD_ERR_NOMOVIEISRECORDED "* No movie is being recorded."
#define TXTCL_CMD_LOG_PRAY "You pray, \"%s\""
#define TXTCL_CMD_LOG_REPORT "You report, \"%s\""
#define TXTCL_CMD_SUBCOMMANDS "  Subcommands:"
#define TXTCL_CMD_CLIENT_COMMANDS "Client commands:"

// Texts used in LaunchURL_cl.cp
#define TXTCL_URL_NO_INTERNETCONFIG "You do not have \"Internet Config\" installed or there was insufficient memory for it to load when " TXTCL_APP_NAME " started. This system extension is required for clickable links to work.\r\r %s"
#define TXTCL_URL_NO_HELPERAPP "You do not have a helper application choosen for the type '%s'.\r\rUse \"Internet Config\" to choose one and restart " TXTCL_APP_NAME " if you would like this link to work.\r\r%s"

// Texts used in Macros_cl.cp
#define TXTCL_MACROS_ERRORCREATINGFILE "Error creating file \"%s\"."
#define TXTCL_MACROS_ERRORCREATINGFOLDER "Error creating \"%s\" folder"
#define TXTCL_MACROS_INFO_UPDATED "\"%s\" updated."
#define TXTCL_MACROS_ERROR_UPDATE "Unable to update \"%s\". It might be open in another application."
#define TXTCL_MACROS_ERROR_OPEN_FILE "Error opening file."
#define TXTCL_MACROS_ERROR_CONTAINS_NULL "File contains a null character. (Did you accidentally save it as UTF-16?)  I'm going to ignore it for now,  but you should fix the file."
#define TXTCL_MACROS_ERROR_MISSING_END_BRACKETS "File is missing end brackets '}'."
#define TXTCL_MACROS_ERROR_OUTOFMEMORY "Out of memory."
#define TXTCL_MACROS_ERROR_PARSINGERROR "Parsing Error."
#define TXTCL_MACROS_ERROR_UNTERMINATEDCOMMENT "Unterminated '/*' comment at end of file."
#define TXTCL_MACROS_ERROR_UNEXPECTEDCLOSINGBRACE "Unexpected closing brace encountered '}'"
#define TXTCL_MACROS_ERROR_STRINGTOOLONG "String is too long: %s"
#define TXTCL_MACROS_ERROR_MATCHINGNOTFOUND "Matching %c not found: %s"
#define TXTCL_MACROS_ERROR_DUPLICATEEXPRESSION "Duplicate expression macro %s."
#define TXTCL_MACROS_ERROR_EXPRESSIONLOADED "Expression macro %s loaded."
#define TXTCL_MACROS_ERROR_DUPLICATEREPLACEMENT "Duplicate replacement macro %s."
#define TXTCL_MACROS_ERROR_REPLACEMENTLOADED "Replacement macro %s loaded."
#define TXTCL_MACROS_ERROR_DUPLICATEINCLUDE "Duplicate include of file %s."
#define TXTCL_MACROS_ERROR_FILEINCLUDED "File \"%s\" included."
#define TXTCL_MACROS_ERROR_DUPLICATECLICK "Duplicate click macro '%s'."
#define TXTCL_MACROS_ERROR_CLICKLOADED "Click macro '%s' loaded."
#define TXTCL_MACROS_ERROR_DUPLICATEMACRO "Duplicate %s macro '%s'."
#define TXTCL_MACROS_ERROR_MACROLOADED "%s macro '%s' loaded."
#define TXTCL_MACROS_ERROR_DUPLICATEFUNCTION "Duplicate function \"%s\"."
#define TXTCL_MACROS_ERROR_FUNCTIONLOADED "Function \"%s\" loaded."
#define TXTCL_MACROS_ERROR_REPLACEMENTTERMINATED "Replacement macro terminated. Pauses are not allowed."
#define TXTCL_MACROS_ERROR_REPLACEMENTWITHRETURN "Execution Error\rReplacement macros may not have a return '\\r' in them."
#define TXTCL_MACROS_ERROR_TEXT_TRUNCATED "Execution Error\rMacro text too long; %u characters truncated.\r"	\
			"Offending text was: \"%s\""
#define TXTCL_MACROS_ERROR_NOTDEFINEDFUNCTION "Execution Error\r'%s' not a defined function."
#define TXTCL_MACROS_ERROR_LABELNOTFOUND "Execution Error\rlabel %s not found in the current macro."

// Texts used in TuneHelper_cl.cp
#define TXTCL_TUNE_ERROR_NO "No Error"
#define TXTCL_TUNE_ERROR_INV_NOTE "The music contains an invalid character"
#define TXTCL_TUNE_ERROR_INV_MODIFIER "The music contains an invalid note modifier"
#define TXTCL_TUNE_ERROR_TONEOVERFLOW "A note is out of the allowed octaves"
#define TXTCL_TUNE_ERROR_POLYPHONYOVERFLOW "Too many chord notes are playing for this instrument"
#define TXTCL_TUNE_ERROR_INV_OCTAVE "A note is out of the allowed octaves"
#define TXTCL_TUNE_ERROR_INV_CHORD "Too many chord notes are playing for this instrument"
#define TXTCL_TUNE_ERROR_UNSUPPORTED_INSTRUMENT "Long chords are not supported on this instrument"
#define TXTCL_TUNE_ERROR_OUTOFMEMORY "Ran out of memory while building tune"
#define TXTCL_TUNE_ERROR_TOMANYMARKS "Loops are nested too deeply"
#define TXTCL_TUNE_ERROR_UNMATCHEDMARK "There is an end loop mark without a start"
#define TXTCL_TUNE_ERROR_UNMATCHEDCOMMENT "Invalid comment termination"
#define TXTCL_TUNE_ERROR_INV_TEMPO "A tempo is outside the allowed values"
#define TXTCL_TUNE_ERROR_INV_TEMPCHANGE "The tempo cannot change while chords are playing"
#define TXTCL_TUNE_ERROR_MODIFIERNEEDVALUE "A tempo modifier is missing its value"
#define TXTCL_TUNE_ERROR_DUPLICATEENDING "An ending number is defined more than once"
#define TXTCL_TUNE_ERROR_DUPLICATEDEFAULTENDING "A default ending is defined more than once"
#define TXTCL_TUNE_ERROR_ENDINGINCHORD "Ending marks cannot be specified in chords"
#define TXTCL_TUNE_ERROR_ENDINGOUTSIDELOOP "Ending marks cannot be specified outside loops"
#define TXTCL_TUNE_ERROR_DEFAULTENDINGERROR "Default endings cannot have an index"
#define TXTCL_TUNE_ERROR_INV_ENDINGINDEX "An ending index is not valid"
#define TXTCL_TUNE_ERROR_UNTERMINATEDLOOP "A loop is missing its end mark"
#define TXTCL_TUNE_ERROR_UNTERMINATEDCHORD "A chord is missing its end mark"
#define TXTCL_TUNE_ERROR_ALREADYPLAYING "You are already playing"

// Texts used in DownloadURL_cl.cp
#define TXTCL_CHECKING_FOR_CLIENT_UPDATE BULLET " Checking for program updates... refer to %s for help troubleshooting."
#define TXTCL_FAILED_CLIENT_UPDATE "Could not download the new client. %d"
#define TXTCL_CHECKING_FOR_DATA_UPDATE BULLET " Checking for data updates... refer to %s for help troubleshooting."
#define TXTCL_FAILED_DATA_UPDATE "Could not download data files. %d"
#define TXTCL_CORRUPT_UPDATE_FILES "The downloaded update files seem to be corrupt. Please contact joe@deltatao.com."
#define TXTCL_DATA_FILES_SUCCESS "Successfully downloaded data files."
#define TXTCL_DOWNLOADING_DATA BULLET " Downloading updater data..."
#define TXTCL_UNABLE_CREATE_DOWNLOAD_DIR "Unable to create a download directory. (%d)"
#define TXTCL_DECOMPRESS_UPDATE_FILE BULLET " Decompressing update file..."

// Texts used in GameWin_cl.cp
#define TXTCL_LANGTABLE1_GIBBERISH "gibberish"
#define TXTCL_LANGTABLE1_HALFLING "Halfling"
#define TXTCL_LANGTABLE1_SYLVAN "Sylvan"
#define TXTCL_LANGTABLE1_PEOPLE "People"
#define TXTCL_LANGTABLE1_THOOM "Thoom"
#define TXTCL_LANGTABLE1_DWARVEN "Dwarven"
#define TXTCL_LANGTABLE1_GHORAK_ZO "Ghorak Zo"
#define TXTCL_LANGTABLE1_ANCIENT "Ancient"
#define TXTCL_LANGTABLE1_MAGIC "magic"
#define TXTCL_LANGTABLE1_COMMON "common"
#define TXTCL_LANGTABLE1_CODE "code"
#define TXTCL_LANGTABLE1_LORE "lore"

#define TXTCL_LANGTABLE2_GIBBERISH "the language of the mad"
#define TXTCL_LANGTABLE2_HALFLING "the Halfling language"
#define TXTCL_LANGTABLE2_SYLVAN "the Sylvan language"
#define TXTCL_LANGTABLE2_PEOPLE "the language of The People"
#define TXTCL_LANGTABLE2_THOOM "the Thoom language"
#define TXTCL_LANGTABLE2_DWARVEN "the Dwarven language"
#define TXTCL_LANGTABLE2_GHORAK_ZO "the Ghorak Zo language"
#define TXTCL_LANGTABLE2_ANCIENT "an ancient language"
#define TXTCL_LANGTABLE2_MAGIC "the language of magic"
#define TXTCL_LANGTABLE2_COMMON "the common language"
#define TXTCL_LANGTABLE2_CODE "a secret language"
#define TXTCL_LANGTABLE2_LORE "a mystical language"

#define TXTCL_LANGTABLE3_GIBBERISH "babbles"
#define TXTCL_LANGTABLE3_HALFLING "squeaks"
#define TXTCL_LANGTABLE3_SYLVAN "chirps"
#define TXTCL_LANGTABLE3_PEOPLE "purrs"
#define TXTCL_LANGTABLE3_THOOM "hums"
#define TXTCL_LANGTABLE3_DWARVEN "mutters"
#define TXTCL_LANGTABLE3_GHORAK_ZO "rumbles"
#define TXTCL_LANGTABLE3_ANCIENT "chants"
#define TXTCL_LANGTABLE3_MAGIC "utters"
#define TXTCL_LANGTABLE3_COMMON "says something"
#define TXTCL_LANGTABLE3_CODE "gestures"
#define TXTCL_LANGTABLE3_LORE "incants"

#define TXTCL_LANGTABLE4_GIBBERISH "babbles softly"
#define TXTCL_LANGTABLE4_HALFLING "squeaks softly"
#define TXTCL_LANGTABLE4_SYLVAN "chirps softly"
#define TXTCL_LANGTABLE4_PEOPLE "purrs softly"
#define TXTCL_LANGTABLE4_THOOM "hums softly"
#define TXTCL_LANGTABLE4_DWARVEN "mumbles"
#define TXTCL_LANGTABLE4_GHORAK_ZO "murmurs"
#define TXTCL_LANGTABLE4_ANCIENT "chants softly"
#define TXTCL_LANGTABLE4_MAGIC "utters softly"
#define TXTCL_LANGTABLE4_COMMON "whispers something"
#define TXTCL_LANGTABLE4_CODE "gestures discreetly"
#define TXTCL_LANGTABLE4_LORE "incants softly"

#define TXTCL_LANGTABLE5_GIBBERISH "Whoop"
#define TXTCL_LANGTABLE5_HALFLING "Squeak"
#define TXTCL_LANGTABLE5_SYLVAN "Chirp"
#define TXTCL_LANGTABLE5_PEOPLE "Roar"
#define TXTCL_LANGTABLE5_THOOM "Thoom"
#define TXTCL_LANGTABLE5_DWARVEN "Humph"
#define TXTCL_LANGTABLE5_GHORAK_ZO "Roar"
#define TXTCL_LANGTABLE5_ANCIENT "..."
#define TXTCL_LANGTABLE5_MAGIC "..."
#define TXTCL_LANGTABLE5_COMMON "Yo"
#define TXTCL_LANGTABLE5_CODE "..."
#define TXTCL_LANGTABLE5_LORE "..."

#define TXTCL_LANGTABLE6_GIBBERISH "Oh"
#define TXTCL_LANGTABLE6_HALFLING "Squeal"
#define TXTCL_LANGTABLE6_SYLVAN "Trill"
#define TXTCL_LANGTABLE6_PEOPLE "Yowl"
#define TXTCL_LANGTABLE6_THOOM "Thooom"
#define TXTCL_LANGTABLE6_DWARVEN "Whoop"
#define TXTCL_LANGTABLE6_GHORAK_ZO "Ooga"
#define TXTCL_LANGTABLE6_ANCIENT "..."
#define TXTCL_LANGTABLE6_MAGIC "..."
#define TXTCL_LANGTABLE6_COMMON "Hey"
#define TXTCL_LANGTABLE6_CODE "..."
#define TXTCL_LANGTABLE6_LORE "..."

#define TXTCL_LANGTABLE7_GIBBERISH "Hi"
#define TXTCL_LANGTABLE7_HALFLING "Yipe"
#define TXTCL_LANGTABLE7_SYLVAN "Hoot"
#define TXTCL_LANGTABLE7_PEOPLE "Snarl"
#define TXTCL_LANGTABLE7_THOOM "Thoooom"
#define TXTCL_LANGTABLE7_DWARVEN "Beer"
#define TXTCL_LANGTABLE7_GHORAK_ZO "Hork"
#define TXTCL_LANGTABLE7_ANCIENT "..."
#define TXTCL_LANGTABLE7_MAGIC "..."
#define TXTCL_LANGTABLE7_COMMON "..."
#define TXTCL_LANGTABLE7_CODE "..."
#define TXTCL_LANGTABLE7_LORE "..."

#define TXTCL_LANGTABLE8_GIBBERISH "shrieks"
#define TXTCL_LANGTABLE8_HALFLING "shouts"
#define TXTCL_LANGTABLE8_SYLVAN "calls"
#define TXTCL_LANGTABLE8_PEOPLE "roars"
#define TXTCL_LANGTABLE8_THOOM "trumpets"
#define TXTCL_LANGTABLE8_DWARVEN "hollers"
#define TXTCL_LANGTABLE8_GHORAK_ZO "bellows"
#define TXTCL_LANGTABLE8_ANCIENT "chants loudly"
#define TXTCL_LANGTABLE8_MAGIC "utters loudly"
#define TXTCL_LANGTABLE8_COMMON "yells something"
#define TXTCL_LANGTABLE8_CODE "gestures wildly"
#define TXTCL_LANGTABLE8_LORE "incants loudly"

#define TXTCL_DATAFILES_MISSING_AUTO "\r" BULLET " The data files are missing. They should automatically download the first time you try to join the game. If not, please go to https://www.deltatao.com/clanlord/trouble.html\r"

#define TXTCL_BUBBLE_UNKNOWN_MONSTER "A monster"
#define TXTCL_BUBBLE_UNKNOWN_PLAYER "Someone"
#define TXTCL_BUBBLE_UNKNOWN_SOMETHING "Something"

#define TXTCL_BUBBLE_ASK "asks"
#define TXTCL_BUBBLE_EXCLAIM "exclaims"
#define TXTCL_BUBBLE_SAY "says"

#define TXTCL_BUBBLE_SAY_IN_LANG "%s %s in %s, \"%s\""
#define TXTCL_BUBBLE_GROWL "%s growls, \"%s\""
#define TXTCL_BUBBLE_WHISPER "%s whispers, \"%s\""
#define TXTCL_BUBBLE_WHISPER_IN_LANG "%s %s in %s, \"%s\""
#define TXTCL_BUBBLE_YELL "%s yells, \"%s\""
#define TXTCL_BUBBLE_YELL_IN_LANG "%s %s in %s, \"%s\"" 
#define TXTCL_BUBBLE_PONDER "%s ponders, \"%s\""

#define TXTCL_BUBBLE_TO_THINK "%s thinks to you, \"%s\""
#define TXTCL_BUBBLE_GROUP_THINK "%s thinks to a group, \"%s\""
#define TXTCL_BUBBLE_CLAN_THINK "%s thinks to your clan, \"%s\""
#define TXTCL_BUBBLE_THINK "%s thinks, \"%s\""

#define TXTCL_BUBBLE_DECLAIM "%s declaims, \"%s\""

#define TXTCL_SENDER_REQUESTS_ATTENTION "%s requests your attention."

#define TXTCL_BUBBLE_UNKNOWN_MEDIUM_IN_LANG "%s %s in %s."
#define TXTCL_BUBBLE_UNKNOWN_LONG_IN_LANG "%s %s in %s."

// Texts used in PlayersWin_cl.cp
#define TXTCL_PLAYERCLASS_EXILE "an exile"
#define TXTCL_PLAYERCLASS_HEALER "a Healer"
#define TXTCL_PLAYERCLASS_FIGHTER "a Fighter"
#define TXTCL_PLAYERCLASS_MYSTIC "a Mystic"
#define TXTCL_PLAYERCLASS_GM "a Game Master"
#define TXTCL_PLAYERCLASS_APMYSTIC "an Apprentice Mystic"
#define TXTCL_PLAYERCLASS_JMMYSTIC "a Journeyman Mystic"
#define TXTCL_PLAYERCLASS_BLOODMAGE "a Bloodmage"
#define TXTCL_PLAYERCLASS_CHAMPION "a Champion"
#define TXTCL_PLAYERCLASS_RANGER "a Ranger"

#define TXTCL_RACE_UNDISCLOSED "not disclosed "
#define TXTCL_RACE_ANCIENT "an Ancient"
#define TXTCL_RACE_HUMAN "a Human"
#define TXTCL_RACE_HALFLING "a Halfling"
#define TXTCL_RACE_SYLVAN "a Sylvan"
#define TXTCL_RACE_PEOPLE "of the People"
#define TXTCL_RACE_THOOM "a Thoom"
#define TXTCL_RACE_DWARF "a Dwarf"
#define TXTCL_RACE_GHORAK_ZO "a Ghorak Zo"

#define TXTCL_GENDER_OTHER "other"
#define TXTCL_GENDER_MALE "male"
#define TXTCL_GENDER_FEMALE "female"
#define TXTCL_GENDER_UNKNOWN "unknown"

#define TXTCL_COLOR_NONE "none"
#define TXTCL_COLOR_RED "red"
#define TXTCL_COLOR_ORANGE "orange"
#define TXTCL_COLOR_GREEN "green"
#define TXTCL_COLOR_BLUE "blue"
#define TXTCL_COLOR_PURPLE "purple"
#define TXTCL_COLOR_BLOCKED "blocked"
#define TXTCL_COLOR_IGNORED "ignored"

#define TXTCL_TITLE_PLAYERS "Players"
#define TXTCL_SAVE_PLAYER_LIST "* Save player list"
#define TXTCL_REBUILD_CL_PLAYERS "Rebuild the CL_Players file?"
#define TXTCL_DBLCLICK_TO_RESET_LIST "Double-click here to rebuild the list"
#define TXTCL_PLAYERS_SHARES "Players: %d Shares: %d/%d"
#define TXTCL_UNIT_SEC "sec"
#define TXTCL_UNIT_MIN "min"
#define TXTCL_UNIT_HOUR "hr"
#define TXTCL_FALLEN_UNKNOWN_LOCATION "Unknown location"
#define TXTCL_FALLEN_BY_UNKNOWN "unknown"

#define TXTCL_FRIENDS_ONLINE "Friends online: "
#define TXTCL_BLOCKED_ONLINE "Blocked players online: "
#define TXTCL_IGNORED_ONLINE "Ignored players online: "
#define TXTCL_FRIENDS_LABELLED_ONLINE "Friends labelled '%s' online: "
#define TXTCL_ONLINE_NONE "none."

#define TXTCL_PURGED_SAVED_PLAYER_LIST "* Purged saved player list"
#define TXTCL_FAILED_ALLOCATE_PLAYER "Failed to allocate memory for a player."

#define TXTCL_IS_NOT_BARD " is not in the Bards' Guild"
#define TXTCL_IS_BARD_GUEST " is a Bard Guest"
#define TXTCL_IS_BARD_QUESTER " is a Bard Quester"
#define TXTCL_IS_BARD " is a Bard"
#define TXTCL_IS_BARD_TRUSTEE " is a Bard Trustee"
#define TXTCL_IS_BARD_MASTER " is a Bard Master"
#define TXTCL_IS_BARD_CRAFTER " is a Bard Crafter"

#define TXTCL_UNKNOWN_LABEL "Unknown label '%s'."
#define TXTCL_LABEL_AMBIGUOUS "Sorry, the label name '%s' is ambiguous."
#define TXTCL_NO_LONGER_BLOCKING "No longer blocking %s."
#define TXTCL_NO_LONGER_IGNORING "No longer ignoring %s."
#define TXTCL_REMOVING_LABEL "Removing label from %s."
#define TXTCL_BLOCKING_PLAYER "Blocking %s."
#define TXTCL_IGNORING_PLAYER "Ignoring %s."
#define TXTCL_LABELLING_PLAYER "Labelling %s %s."

#define TXTCL_PARTIAL_NAME_NOT_UNIQUE "The partial name '%s' is not unique."
#define TXTCL_NO_SUCH_PLAYER_IN_LIST "No player named '%s' found in the player list."

// Texts used in Info_cl.cp
#define TXTCL_CHAR_MANAGER_FAILURE "Sorry, there was a failure in the Character Manager with id %d."
#define TXTCL_MUST_FILL_FIELD "You must fill in this field."
#define TXTCL_CLICK_ON_DEMO_CHARS "Double-click on any of these demo characters:"
#define TXTCL_CHARS_FOR_ACCOUNT "Characters for account " LDQUO "%s" RDQUO ":"
#define TXTCL_PROBLEM_WITH_ACCOUNT "There is a problem with your account.\rPlease contact Delta Tao."
#define TXTCL_PAID_ONE_CHAR "You have paid for one character.\r"
#define TXTCL_PAID_X_CHARS "You have paid for %d characters.\r"
#define TXTCL_ACCOUNT_PAID_UNTIL "Your account is paid until %s."
#define TXTCL_ACCOUNT_EXPIRE_TODAY "Your account expires today."
#define TXTCL_ACCOUNT_EXPIRED_ON "Your account expired on %s."
#define TXTCL_DEMO_CHAR_DESC TXTCL_APP_NAME " demo characters inhabit the magical land of Agratis. Regular players often visit, and you can take a brief tour of the main island."
#define TXTCL_FAILED_CREATE_CHAR "Could not create your new character. Error %d.\r%s"
#define TXTCL_CHAR_CREATED "The character \"%s\" has been created with the same password as your account. Select \"Password...\" to change it for this character."
#define TXTCL_CHAR_DELETED "The character \"%s\" has been deleted."
#define TXTCL_FAILED_DELETE_CHAR "Could not delete the character \"%s\". Error %d.\r%s"
#define TXTCL_PASS_CHANGED "The password for the character \"%s\" has been changed."
#define TXTCL_FAILED_CHANGE_PASS "Could not change password for the character \"%s\". Error %d.\r%s"
#define TXTCL_PASS_NOT_SAME "Passwords are not the same! Please try again."

// Texts used in InvenWin_cl.cp
#define TXTCL_INVENTORY_WIN_TITLE "Inventory"
#define TXTCL_UNNAMED_ITEM "<unnamed item>"
#define TXTCL_UNEQUIP "Unequip"
#define TXTCL_EQUIP "Equip"
#define TXTCL_USE "Use"
#define TXTCL_USE_ITEM "Use %s"
#define TXTCL_NR_ITEMS "%d items. "
#define TXTCL_ONE_SLOT_FREE "%d slot free"
#define TXTCL_X_SLOTS_FREE "%d slots free"
#define TXTCL_PACK_FULL "Your pack is full."

#define TXTCL_SLOTNAME_INVALID "Invalid"
#define TXTCL_SLOTNAME_UNKNOWN "unknown"
#define TXTCL_SLOTNAME_FOREHEAD "forehead"
#define TXTCL_SLOTNAME_NECK "neck"
#define TXTCL_SLOTNAME_SHOULDER "shoulder"
#define TXTCL_SLOTNAME_ARMS "arms"
#define TXTCL_SLOTNAME_GLOVES "gloves"
#define TXTCL_SLOTNAME_FINGER "finger"
#define TXTCL_SLOTNAME_COAT "coat"
#define TXTCL_SLOTNAME_CLOAK "cloak"
#define TXTCL_SLOTNAME_TORSO "torso"
#define TXTCL_SLOTNAME_WAIST "waist"
#define TXTCL_SLOTNAME_LEGS "legs"
#define TXTCL_SLOTNAME_FEET "feet"
#define TXTCL_SLOTNAME_RIGHT "right"
#define TXTCL_SLOTNAME_LEFT "left"
#define TXTCL_SLOTNAME_HANDS "hands"
#define TXTCL_SLOTNAME_HEAD "head"

#define TXTCL_ITEMNAME_NOTHING "Nothing"
#define TXTCL_NO_SUCH_ITEM_IN_LIST "No item named '%s' found in the item list."
#define TXTCL_WIELDING_ITEM "You wield your %s."
#define TXTCL_INVENTORY_CMD_IN_PROGRESS "An inventory command is already in progress."
#define TXTCL_NO_ITEM_BY_THAT_NAME "No item by that name."
#define TXTCL_UNKNOWN_ERROR "Unknown error"
#define TXTCL_NOT_CONNECTED "You are not connected to the game."

// Texts used in Music_cl.cp -- obsolete
//#define TXTCL_STARTING_MUSIC_FILE "[Starting music file: " LDQUO "%.64s" RDQUO "]"

// Texts used in TextPrefs_cl.cp
#define TXTCL_SAMPLETEXT_PLAIN "This is plain text as it would appear in your info bar."
#define TXTCL_SAMPLETEXT_STRANGER_CLANNING "Random Stranger is now Clanning."
#define TXTCL_SAMPLETEXT_FRIEND_CLANNING "Best Friend is now Clanning."
#define TXTCL_SAMPLETEXT_STRANGER_NO_CLANNING "Random Stranger is no longer Clanning."
#define TXTCL_SAMPLETEXT_FRIEND_NO_CLANNING "Best Friend is no longer Clanning."
#define TXTCL_SAMPLETEXT_STRANGER_SHARE "Random Stranger is sharing experiences with you."
#define TXTCL_SAMPLETEXT_FRIEND_SHARE "Best Friend is sharing experiences with you."
#define TXTCL_SAMPLETEXT_ANNOUNCE "* You gain experience!"
#define TXTCL_SAMPLETEXT_STRANGER_FALLEN "Random Stranger has fallen to a Rat in Puddleby."
#define TXTCL_SAMPLETEXT_FRIEND_FALLEN "Best Friend has fallen to an Orga Fury in the Orga Camp."
#define TXTCL_SAMPLETEXT_STRANGER_TALK "Random Stranger says, \"Nice to see you.\""
#define TXTCL_SAMPLETEXT_FRIEND_TALK "Best Friend asks, \"Hey, how've you been?\""
#define TXTCL_SAMPLETEXT_MYSELF_TALK "You exclaim, \"I passed the circle test!\""
#define TXTCL_SAMPLETEXT_BUBBLES "Dang is the marsh!"
#define TXTCL_SAMPLETEXT_FRIEND_BUBBLES "You thing rike jellyfish."
#define TXTCL_SAMPLETEXT_THINKTO "Someone thinks to you, \"Congrats!\""
#define TXTCL_SAMPLETEXT_TIMESTAMP "12/25/01 12:34:56p"

#define TXTCL_CHOOSE_COLOR "Choose a color for: %s"
#define TXTCL_UNNAMED_STYLE "<unnamed style %d>"

// Texts used in TextWin_cl.cp
#define TXTCL_TITLE_TEXT "Text"

// Texts used in Update_cl.cp
#define TXTCL_UNABLE_OPEN_PREFS "Unable to open preferences file. (Is your disk locked?)"
#define TXTCL_LOCATE_IMAGE_UPDATE "Please locate the images update files."
#define TXTCL_LOCATE_SOUND_UPDATE "Please locate the sounds update files."
#define TXTCL_CLIENT_OLDER_THAN_DATA "This client (version %s) should not be used with this newer data file (file " LDQUO "%s" RDQUO " is version %s)."
#define TXTCL_UNABLE_UPDATE_DATA "Unable to update the data files to version %d. Make sure the file " LDQUO "%s.%dto%d" RDQUO " (or an alias) is in the same folder as " LDQUO TXTCL_APP_NAME " v%d" RDQUO "."
#define TXTCL_UPDATING_SOMETHING "Updating %s..." 
#define TXTCL_UPDATING_IMAGES "images"
#define TXTCL_UPDATING_SOUNDS "sounds"
#define TXTCL_UPDATING_DATA "data"
#define TXTCL_PROGRESS_THOUSAND_SEPARATOR ","

// Texts used in OpenGL_CL.r (and GUI sizes) -- obsolete
#if 0
#define TXTCL_OGL_USE_OGL "Use OpenGL"
#define TXTCL_OGL_USE_OGL_COORD1 160
#define TXTCL_OGL_USE_OGL_COORD2 23
#define TXTCL_OGL_USE_OGL_COORD3 264
#define TXTCL_OGL_USE_OGL_COORD4 303 
#define TXTCL_OGL_OK "OK"
#define TXTCL_OGL_OK_COORD1 391
#define TXTCL_OGL_OK_COORD2 232 
#define TXTCL_OGL_OK_COORD3 411
#define TXTCL_OGL_OK_COORD4 301
#define TXTCL_OGL_CANCEL "Cancel"
#define TXTCL_OGL_CANCEL_COORD1 391
#define TXTCL_OGL_CANCEL_COORD2 151
#define TXTCL_OGL_CANCEL_COORD3 411
#define TXTCL_OGL_CANCEL_COORD4 220
#define TXTCL_OGL_COLORS "Colors:"
#define TXTCL_OGL_COLORS_COORD1 49
#define TXTCL_OGL_COLORS_COORD2 34 
#define TXTCL_OGL_COLORS_COORD3 65 
#define TXTCL_OGL_COLORS_COORD4 154
#define TXTCL_OGL_BYBARTYPE "by bar type"
#define TXTCL_OGL_BYBARTYPE_COORD1 66
#define TXTCL_OGL_BYBARTYPE_COORD2 34
#define TXTCL_OGL_BYBARTYPE_COORD3 84
#define TXTCL_OGL_BYBARTYPE_COORD4 154
#define TXTCL_OGL_BYBARVALUE "by bar value"
#define TXTCL_OGL_BYBARVALUE_COORD1 85
#define TXTCL_OGL_BYBARVALUE_COORD2 34 
#define TXTCL_OGL_BYBARVALUE_COORD3 103 
#define TXTCL_OGL_BYBARVALUE_COORD4 154
#define TXTCL_OGL_PLACEMENT "Placement:"
#define TXTCL_OGL_PLACEMENT_COORD1 49
#define TXTCL_OGL_PLACEMENT_COORD2 170 
#define TXTCL_OGL_PLACEMENT_COORD3 65 
#define TXTCL_OGL_PLACEMENT_COORD4 294 
#define TXTCL_OGL_ACROSSBOTTOM "Across Bottom"
#define TXTCL_OGL_AB_C1 66
#define TXTCL_OGL_AB_C2 170 
#define TXTCL_OGL_AB_C3 84 
#define TXTCL_OGL_AB_C4 294 
#define TXTCL_OGL_LOWERLEFT "Lower Left"
#define TXTCL_OGL_LL_C1 85 
#define TXTCL_OGL_LL_C2 170 
#define TXTCL_OGL_LL_C3 103
#define TXTCL_OGL_LL_C4 294 
#define TXTCL_OGL_LOWERRIGHT "Lower Right"
#define TXTCL_OGL_LR_C1 104 
#define TXTCL_OGL_LR_C2 170 
#define TXTCL_OGL_LR_C3 122
#define TXTCL_OGL_LR_C4 294 
#define TXTCL_OGL_UPPERRIGHT "Upper Right"
#define TXTCL_OGL_UR_C1 123
#define TXTCL_OGL_UR_C2 170 
#define TXTCL_OGL_UR_C3 141
#define TXTCL_OGL_UR_C4 294 
#define TXTCL_OGL_NOTIFYWHENFRIENDSTHINK "Notify when friends think to you"
#define TXTCL_OGL_NT_C1 278 
#define TXTCL_OGL_NT_C2 36 
#define TXTCL_OGL_NT_C3 296 
#define TXTCL_OGL_NT_C4 286 
#define TXTCL_OGL_TREATSHARESASFRIENDS "Treat shares as friends"
#define TXTCL_OGL_TS_C1 298 
#define TXTCL_OGL_TS_C2 36 
#define TXTCL_OGL_TS_C3 316 
#define TXTCL_OGL_TS_C4 286 
#define TXTCL_OGL_IGNORECMDQ "Ignore Command-Q menu key"
#define TXTCL_OGL_IQ_C1 318 
#define TXTCL_OGL_IQ_C2 36 
#define TXTCL_OGL_IQ_C3 336
#define TXTCL_OGL_IQ_C4 286 
#define TXTCL_OGL_STARTLOGONJOIN "Start new text log on every Join"
#define TXTCL_OGL_LJ_C1 338 
#define TXTCL_OGL_LJ_C2 36 
#define TXTCL_OGL_LJ_C3 356
#define TXTCL_OGL_LJ_C4 286 
#define TXTCL_OGL_NEVERLOGFORMMOVIES "Never make text logs from movies"
#define TXTCL_OGL_NM_C1 358 
#define TXTCL_OGL_NM_C2 36 
#define TXTCL_OGL_NM_C3 376
#define TXTCL_OGL_NM_C4 286 
#define TXTCL_OGL_USEBESTRENDERER "Use best renderer available"
#define TXTCL_OGL_UB_C1 183 
#define TXTCL_OGL_UB_C2 34 
#define TXTCL_OGL_UB_C3 201 
#define TXTCL_OGL_UB_C4 284 
#define TXTCL_OGL_IGNOREHARDWAREACC "Ignore hardware accelerator"
#define TXTCL_OGL_IA_C1 202 
#define TXTCL_OGL_IA_C2 34 
#define TXTCL_OGL_IA_C3 220
#define TXTCL_OGL_IA_C4 284 
#define TXTCL_OGL_ALLOLDACC "Allow old accelerators, if possible"
#define TXTCL_OGL_AO_C1 221
#define TXTCL_OGL_AO_C2 34 
#define TXTCL_OGL_AO_C3 239
#define TXTCL_OGL_AO_C4 284 
#define TXTCL_OGL_SUBSTEFFECTS "Substitute OpenGL special effects"
#define TXTCL_OGL_SE_C1 240
#define TXTCL_OGL_SE_C2 34 
#define TXTCL_OGL_SE_C3 258
#define TXTCL_OGL_SE_C4 284 
#define TXTCL_OGL_ADVPREFS "Advanced Preferences"
#define TXTCL_OGL_AP_C1 108 
#define TXTCL_OGL_AP_C2 158 
#define TXTCL_OGL_AP_C3 543 
#define TXTCL_OGL_AP_C4 484 
#define TXTCL_OGL_BOX_C1 160 
#define TXTCL_OGL_BOX_C2 23 
#define TXTCL_OGL_BOX_C3 264 
#define TXTCL_OGL_BOX_C4 303
#endif  // 0
