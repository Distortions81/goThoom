/*
**	Commands_cl.cp		ClanLord
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


#include "ClanLord.h"
#include "Commands_cl.h"
#include "Movie_cl.h"


#define COMMAND_MANY_PREFS	1

// Groups of commands which form a hierarchical tree, including the /help for them.

#define COMMAND_GROUP_TERMINATOR	{ nullptr, CommandDefinition::None, nullptr, nullptr }


const CommandDefinition
gPrefMovementCommandDefs[] =
{
	{ "HOLD",	CommandDefinition::PrefMovementHold, 	nullptr,	TXTCL_CMD_HELP_HOLD },
	{ "TOGGLE",	CommandDefinition::PrefMovementToggle, 	nullptr,	TXTCL_CMD_HELP_TOGGLE },
	COMMAND_GROUP_TERMINATOR
};


const CommandDefinition
gPrefMessageCommandDefs[] =
{
	{ "CLANNING",	CommandDefinition::PrefMessageClanning, nullptr,	TXTCL_CMD_HELP_CLANNING },
	{ "FALLEN",		CommandDefinition::PrefMessageFallen, 	nullptr,	TXTCL_CMD_HELP_FALLEN },
	COMMAND_GROUP_TERMINATOR
};


const CommandDefinition
gPrefCommandDefs[] =
{
	{ "MOVEMENT",	CommandDefinition::PrefMovement,	gPrefMovementCommandDefs,
		TXTCL_CMD_HELP_MOVEMENT },
	
#if COMMAND_MANY_PREFS
//	{ "LARGEWINDOW",	CommandDefinition::PrefLargeWindow, nullptr,	TXTCL_CMD_HELP_LARGEWINDOW },
	{ "MESSAGE",		CommandDefinition::PrefMessage,		gPrefMessageCommandDefs,
																	TXTCL_CMD_HELP_MESSAGE },
	{ "BRIGHTCOLORS",	CommandDefinition::PrefBrightColors, nullptr,	TXTCL_CMD_HELP_BRIGHTCOLORS },
	
#ifdef AUTO_HIDENAME
	{ "AUTOHIDE",		CommandDefinition::PrefShowNames,	nullptr,	TXTCL_CMD_HELP_AUTOHIDE },
#else
	{ "SHOWNAMES",		CommandDefinition::PrefShowNames, 	nullptr,	TXTCL_CMD_HELP_SHOWNAMES },
#endif	// AUTO_HIDENAME
	
	{ "TIMESTAMPS",		CommandDefinition::PrefTimeStamps, 		nullptr,  TXTCL_CMD_HELP_TIMESTAMPS },
	{ "MAXNIGHTPERCENT", CommandDefinition::PrefMaxNightPercent, nullptr,
																TXTCL_CMD_HELP_MAXNIGHTPERCENT },
	{ "VOLUME",			CommandDefinition::PrefSoundVolume,		nullptr,	TXTCL_CMD_HELP_VOLUME },
	{ "BARDVOLUME",		CommandDefinition::PrefBardVolume,		nullptr,  TXTCL_CMD_HELP_BARDVOLUME },
	{ "NEWLOG",			CommandDefinition::PrefNewLogEveryJoin,	nullptr,	TXTCL_CMD_HELP_NEWLOG },
	{ "MOVIELOGS",		CommandDefinition::PrefNoMovieTextLogs,	nullptr,	TXTCL_CMD_HELP_MOVIELOGS },
#endif	// COMMAND_MANY_PREFS
	
	COMMAND_GROUP_TERMINATOR
};


const CommandDefinition
gSelectCommandDefs[] =
{
	{ "@NEXT",	CommandDefinition::SelectNext, 		nullptr,	"" },
	{ "@PREV",	CommandDefinition::SelectPrevious, 	nullptr,	"" },
	{ "@FIRST",	CommandDefinition::SelectFirst, 	nullptr,	"" },
	{ "@LAST",	CommandDefinition::SelectLast, 		nullptr,	"" },
	
	COMMAND_GROUP_TERMINATOR
};


const CommandDefinition
gMoveCommandDefs[] =
{
	{ "N",			CommandDefinition::Move + move_North, 		nullptr,	"" },
	{ "NORTH",		CommandDefinition::Move + move_North, 		nullptr,	"" },
	{ "NE",			CommandDefinition::Move + move_NorthEast, 	nullptr,	"" },
	{ "NORTHEAST",	CommandDefinition::Move + move_NorthEast, 	nullptr,	"" },
	{ "E",			CommandDefinition::Move + move_East, 		nullptr,	"" },
	{ "EAST",		CommandDefinition::Move + move_East, 		nullptr,	"" },
	{ "SE",			CommandDefinition::Move + move_SouthEast, 	nullptr,	"" },
	{ "SOUTHEAST",	CommandDefinition::Move + move_SouthEast, 	nullptr,	"" },
	{ "S",			CommandDefinition::Move + move_South, 		nullptr,	"" },
	{ "SOUTH",		CommandDefinition::Move + move_South, 		nullptr,	"" },
	{ "SW",			CommandDefinition::Move + move_SouthWest, 	nullptr,	"" },
	{ "SOUTHWEST",	CommandDefinition::Move + move_SouthWest, 	nullptr,	"" },
	{ "W",			CommandDefinition::Move + move_West, 		nullptr,	"" },
	{ "WEST",		CommandDefinition::Move + move_West, 		nullptr,	"" },
	{ "NW",			CommandDefinition::Move + move_NorthWest, 	nullptr,	"" },
	{ "NORTHWEST",	CommandDefinition::Move + move_NorthWest, 	nullptr,	"" },
	{ "STOP",		CommandDefinition::Move + move_Stop, 		nullptr,	"" },
	COMMAND_GROUP_TERMINATOR
};


const CommandDefinition
gMoveSpeedCommandDefs[] =
{
	{ "RUN",	move_speed_Run, 	nullptr,	"" },
	{ "WALK",	move_speed_Walk, 	nullptr,	"" },
	{ "STOP",	move_speed_Stop, 	nullptr,	"" },
	COMMAND_GROUP_TERMINATOR
};


// Base level commands
const CommandDefinition
gCommandDefs[] =
{
	{ "LABEL",		CommandDefinition::Label, 		nullptr,			TXTCL_CMD_HELP_LABEL },
	{ "BLOCK",		CommandDefinition::Block, 		nullptr,			TXTCL_CMD_HELP_BLOCK },
	{ "FORGET",		CommandDefinition::Forget, 		nullptr,			TXTCL_CMD_HELP_FORGET },
	{ "IGNORE",		CommandDefinition::Ignore, 		nullptr,			TXTCL_CMD_HELP_IGNORE },
	{ "MOVE",		CommandDefinition::Move,		gMoveCommandDefs,	TXTCL_CMD_HELP_MOVE },
	{ "PREF",		CommandDefinition::Pref, 		gPrefCommandDefs,	TXTCL_CMD_HELP_PREF },
	{ "RECORD",		CommandDefinition::RecordMovie,	nullptr,			TXTCL_CMD_HELP_RECORD },
	{ "SELECT",		CommandDefinition::Select, 		nullptr,			TXTCL_CMD_HELP_SELECT },
	{ "SELECTITEM",	CommandDefinition::SelectItem,	nullptr,			TXTCL_CMD_HELP_SELECTITEM },
	{ "WHOLABEL",	CommandDefinition::WhoFriend,	nullptr,			TXTCL_CMD_HELP_WHOLABEL },
	
	COMMAND_GROUP_TERMINATOR
};


// A level above base level commands for resolving helps
const CommandDefinition
gHelpCommandDefs[] =
{
	{ "HELP", 		CommandDefinition::Help, 	gCommandDefs,	"" },
	COMMAND_GROUP_TERMINATOR
};


// Server commands that need to be logged
const CommandDefinition
gServerCommandDefs[] =
{
	{ "THINKTO", 	CommandDefinition::Thinkto, 	nullptr,	"" },
	{ "BUG", 		CommandDefinition::Bug, 		nullptr,	"" },
	{ "PRAY", 		CommandDefinition::Pray, 		nullptr,	"" },
#ifdef DEBUG_VERSION
	{ "USE",		CommandDefinition::Use,			nullptr,	"" },
	{ "USEITEM",	CommandDefinition::UseItem,		nullptr,	"" },
	{ "MESSAGE",	CommandDefinition::Message,		nullptr,	"" },
#endif	// defined( DEBUG_VERSION )
	
	COMMAND_GROUP_TERMINATOR
};



// only here to de-discombobulate the Xcode3 syntax colorizer
struct CommandDefinition;


// CommandDefinition::ResolveCommand()
//
// Recursive function which goes through words of the command comparing
// on each level and returning the command
int
CommandDefinition::ResolveCommand( const CommandDefinition ** def, SafeString * inputStr)
{
	SafeString cmdStr;
	ClientCommand::GetWord( inputStr, &cmdStr );
	
//	bool found = false;
	int ii = 0;
	while ( (*def)[ ii ].cdKeyword )
		{
		if ( 0 == strcasecmp( cmdStr.Get(), (*def)[ ii ].cdKeyword ) )
			{
//			found = true;
			*def = &(*def)[ ii ];
			if ( (*def)->cdSubCommands )
				{
				const CommandDefinition * sendDef = (*def)->cdSubCommands;
				int cmdID = ResolveCommand( &sendDef, inputStr );
				if ( sendDef )
					{
					*def = sendDef;
					return cmdID;
					}
				
				return 0;
				}
			
			// This is a valid command, what is left in inputStr is a variable parameter
			return (*def)->cdID;
			}
		++ii;
		}
	
//	if ( not found )
	*def = nullptr;
	
	return 0;
}


namespace ClientCommand
{
/*
**	ClientCommand::GetWord()
**	extract one word from the input line
*/
void
GetWord( SafeString * inLine, SafeString * outDest )
{
	const uchar * p = reinterpret_cast<uchar *>( inLine->Get() );
	const char * const breaks = " \r\n";
	char quote = 0;
	bool done = false;
	
	outDest->Clear();
	
	// Degenerate case
	if ( not p )
		return;
	
	// Skip initial spaces
	while ( *p && isspace( *p ) )
		++p;
	
	// See if the first character is a quote or apostrophe
	if ( *p && strchr( "'\"", *p ) )
		{
		quote = *p;
		++p;
		}
	
	// Go until a matching quote, or a break, or the end
	while ( *p && not done )
		{
		if ( *p == quote )
			{
			++p;
			done = true;
			}
		else
			{
			// Handle separators when not in quotes, and carriage returns always
			if ( not quote && strchr( breaks, *p ) )
				done = true;
			else
				outDest->Format( "%c" , *p++ );
			}
		}
	
	// Skip ending spaces
	while ( *p && isspace( *p ) )
		++p;
	
	inLine->Set( reinterpret_cast<const char *>( p ) );
}


/*
**	ClientCommand::ResolveInt()
**	attempt to extract an integer from the input
*/
bool
ResolveInt( const SafeString * input, int * num, bool showError )
{
	// Make sure the word looks like a long
	const uchar * p = reinterpret_cast<const uchar *>( input->Get() );
	
	// They must all be digits
	bool valid = true;
	while ( *p && valid )
		{
		if ( not isdigit( *p ) )
			valid = false;
		++p;
		}
	
	if ( not valid && showError )
		{
		SafeString errStr;
			/* "'%s' is not a number." */
		errStr.Format( _(TXTCL_CMD_ERR_NONUMBER), input->Get() );
		ShowInfoText( errStr.Get() );
		}
	
	// It is a valid number
	if ( valid )
		sscanf( input->Get(), "%d", num );
	
	return valid;
}


/*
**	ClientCommand::ResolveBoolean()
**	attempt to extract a true/false value from the input
*/
bool
ResolveBoolean( const SafeString * word, bool * theBoolean, bool showError )
{
	const char * p = word->Get();
	
	if ( 0 == strcasecmp( "FALSE", p )
	||	 0 == strcasecmp( "OFF",   p ) )
		{
		*theBoolean = false;
		return true;
		}
	
	if ( 0 == strcasecmp( "TRUE", p )
	||	 0 == strcasecmp( "ON",   p ) )
		{
		*theBoolean = true;
		return true;
		}
	
	int num;
	if ( ResolveInt( word, &num, false ) )
		{
		*theBoolean = (0 != num);
		return true;
		}
	
	if ( showError )
		{
		SafeString errStr;
			/* "'%s' is not a TRUE/FALSE value." */
		errStr.Format( _(TXTCL_CMD_ERR_NOTTRUEFALSE), p );
		ShowInfoText( errStr.Get() );
		}
	
	return false;
}


/*
**	ClientCommand::HandleCommand()
**
** Handle server commands, and tell the client not to send them if it did.
*/
int
HandleCommand( const char * cmd )
{
	// Check to see if the starting character is '/' or '\'
	if ( '\\' != * (const uchar *) cmd  &&  '/' != * (const uchar *) cmd )
		return false;
	
	// Resolve the command without the command mark
	
	// Handle /HELP
	SafeString cmdStr;
	cmdStr.Set( cmd + 1 );
	const CommandDefinition * def = gHelpCommandDefs;
	int cmdID = CommandDefinition::ResolveCommand( &def, &cmdStr );
	
	if ( def )
		{
		// A simple \HELP
		
		if ( gHelpCommandDefs == def )
			{
			cmdStr.Set( cmd + 1 );
			HandleHelp( &cmdStr );
			
			// We need the server commands to be shown too.
			// Or help for a command we don't know about
			return false;
			}
		
		// It is help for one of our commands:
		// send the command help and then a list of subcommands
		HandleHelpCommand( def );
		return kHandled;
		}
	
	// Handle Client Commands
	cmdStr.Set( cmd + 1 );
	def = gCommandDefs;
	cmdID = CommandDefinition::ResolveCommand( &def, &cmdStr );	
	
	if ( not cmdID )
		{
		if ( def )
			{
			// We handle the telling that it isn't a command
			HandleHelpCommand( def );
			return kHandled;
			}
		
// Only do this on an option?
		
		// Log certain server commands! e.g. /thinkto /bug /pray
		// or anything which doesn't echo the text they typed...
		cmdStr.Set( cmd + 1 );
		def = gServerCommandDefs;
		cmdID = CommandDefinition::ResolveCommand( &def, &cmdStr );
		if ( cmdID )
			{
			// this handles commands that need reformatting
			// ex. '\pray blah blah' --> 'You pray, "blah blah"'
			HandleLoggedServerCommand( cmdID, &cmdStr );
			}
		
		// The server handles it, the main command must have been bad or a server command
		return kNotHandled;
		}
	
	// Read the high-word of the ID to get the function to call
	switch ( (cmdID >> 16) & 0x0FFFF )
		{
		case CommandDefinition::CatPlayer:
			HandlePlayerCommand( cmdID, &cmdStr, def );
			break;
		
		case CommandDefinition::CatPref:
			HandlePrefCommand( cmdID, &cmdStr, def );
			break;
		
		case CommandDefinition::CatMove:
			HandleMoveCommand( cmdID, &cmdStr );
			break;
		
#if 0
		case CommandDefinition::CatEquip:
			return HandleEquipCommand( &cmdStr );
			break;
		
		case CommandDefinition::CatUnequip:
			return HandleUnequipCommand( cmdID );
			break;
#endif  // 0
		
		case CommandDefinition::CatSelectItem:
			HandleSelectItemCommand( &cmdStr );
			break;
		
		case CommandDefinition::CatMovie:
			HandleMovieCommand( &cmdStr );
			break;
		}
	
	return kHandled;	
}


/*
**	ClientCommand::HandlePlayerCommand()
*/
void
HandlePlayerCommand( int cmdID, SafeString * cmdStr, const CommandDefinition * def )
{
	// Get the next word
	SafeString word;
	GetWord( cmdStr, &word );
	
	switch ( cmdID )
		{
		// Commands
		case CommandDefinition::Select:
			{
			const CommandDefinition * def2 = gSelectCommandDefs;
			// Check for next/previous
			SafeString word2;
			word2.Set( word.Get() );
			int id2 = CommandDefinition::ResolveCommand( &def2, &word2 );
			
			switch ( id2 )
				{
				case CommandDefinition::SelectNext:
					OffsetSelectedPlayer( 1 );
					break;
				
				case CommandDefinition::SelectPrevious:
					OffsetSelectedPlayer( -1 );
					break;
				
				case CommandDefinition::SelectFirst:
					OffsetSelectedPlayer( (int) 0x80000000 );
					break;
				
				case CommandDefinition::SelectLast:
					OffsetSelectedPlayer( 0x7FFFFFFF );
					break;
				
				default:
					if ( word.Size() > 1 && ResolvePlayerName( &word, true ) )
						SelectPlayer( word.Get(), false );
					else
						SelectPlayer( nullptr, true );
					break;
				}
			}
			break;
		
		case CommandDefinition::Label:
			// handle help
			if ( word.Size() <= 1						// \label
			||	 0 == strcmp( word.Get(), "?" ) ) // \label ?
				{
				HandleHelpCommand( def );
				}
			else
			if ( ResolvePlayerName( &word, true ) )
				{
				// attempt to extract the label name
				SafeString labelStr;
				GetWord( cmdStr, &labelStr );
				if ( strlen( labelStr.Get() ) )
					{
					int labelNum = FriendFlagNumberFromName( labelStr.Get(), false, true );
					if ( labelNum >= 0 )
						SetNamedFriend( word.Get(), labelNum, true );
					}
				else
					{
					// default to label 1
					SetNamedFriend( word.Get(), kFriendLabel1, true );
					}
				}
			break;
		
		case CommandDefinition::Ignore:
			// handle help
			if ( word.Size() <= 1
			||	 0 == strcmp( word.Get(), "?" ) )
				{
				HandleHelpCommand( def );
				}
			else
			if ( ResolvePlayerName( &word, true ) )
				SetNamedFriend( word.Get(), kFriendIgnore, true );
			break;
		
		case CommandDefinition::Block:
			// handle help
			if ( word.Size() <= 1
			||	 0 == strcmp( word.Get(), "?" ) )
				{
				HandleHelpCommand( def );
				}
			else
			if ( ResolvePlayerName( &word, true ) )
				SetNamedFriend( word.Get(), kFriendBlock, true );
			break;
		
		case CommandDefinition::Forget:
			// handle help
			if ( word.Size() <= 1
			||	 0 == strcmp( word.Get(), "?" ) )
				{
				HandleHelpCommand( def );
				}
			else
			if ( ResolvePlayerName( &word, true ) )
				SetNamedFriend( word.Get(), kFriendNone, true );
			break;
		
		case CommandDefinition::WhoFriend:
			// handle help
			if ( 0 == strcmp( word.Get(), "?" ) )
				HandleHelpCommand( def );
			else
			if ( word.Size() <= 1 )
				ListFriends();
			else
				{
				int labelNum = FriendFlagNumberFromName( word.Get(), false, true );
				if ( kFriendNone == labelNum )
					{
					SafeString msg;
					msg.Clear();
						/* "Unknown label '%s'" */
					msg.Format( _(TXTCL_CMD_ERR_UNKNOWNLABEL), word.Get() );
					ShowInfoText( msg.Get() );
					}
				else
				if ( labelNum >= 0 )
					ListFriends( labelNum );
				}
			break;
		}
}


/*
**	ClientCommand::HandlePrefCommand()
*/
void
HandlePrefCommand( int cmdID, SafeString * cmdStr, const CommandDefinition * def )
{
	// Get the next word
	SafeString word;
	GetWord( cmdStr, &word );
	
	// scratch vars
	bool theBool;
	bool bGotParam = true;
	int value;
	
	switch ( cmdID )
		{
		
// simple, single-valued preferences
		case CommandDefinition::PrefMovementHold:
			gPrefsData.pdMoveControls = kMoveClickHold;
			gClickState = 0;
			break;
		
		case CommandDefinition::PrefMovementToggle:
			gPrefsData.pdMoveControls = kMoveClickToggle;
			gClickState = 0;
			break;
		
		
#if COMMAND_MANY_PREFS
// more complex preferences, which require parameters
		
		case CommandDefinition::PrefShowNames:
			bGotParam = ResolveBoolean( &word, &theBool, true );
			if ( bGotParam )
				gPrefsData.pdShowNames = theBool;
			break;
		
		case CommandDefinition::PrefTimeStamps:
			bGotParam = ResolveBoolean( &word, &theBool, true );
			if ( bGotParam )
				gPrefsData.pdTimeStamp = theBool;
			break;
		
		case CommandDefinition::PrefBrightColors:
			bGotParam = ResolveBoolean( &word, &theBool, true );
			if ( bGotParam )
				{
				gPrefsData.pdBrightColors = theBool;
				SetBackColors();
				}
			break;
		
		case CommandDefinition::PrefMessageFallen:
			bGotParam = ResolveBoolean( &word, &theBool, true );
			if ( bGotParam )
				gPrefsData.pdSuppressFallen = not theBool;
			break;
		
		case CommandDefinition::PrefMessageClanning:
			bGotParam = ResolveBoolean( &word, &theBool, true );
			if ( bGotParam )
				gPrefsData.pdSuppressClanning = not theBool;
			break;
		
		case CommandDefinition::PrefMaxNightPercent:
			bGotParam = ResolveInt( &word, &value, true );
			if ( bGotParam )
				{
				if ( value > 100 )
					value = 100;
				else
				if ( value < 0 )
					value = 0;
				// Round the value to the nearest 25% to be the same as the prefs dialog
				value = (value / 25) * 25;
				gPrefsData.pdMaxNightPct = value;
				}
			break;
		
		case CommandDefinition::PrefSoundVolume:
		case CommandDefinition::PrefBardVolume:
			bGotParam = ResolveInt( &word, &value, true );
			if ( bGotParam )
				{
				if ( value < 0 )
					value = 0;
				else
				if ( value > 100 )
					value = 100;
				if ( CommandDefinition::PrefSoundVolume == cmdID )
					{
					gPrefsData.pdSoundVolume = value;
					SetSoundVolume( gPrefsData.pdSoundVolume );
					}
				else
					SetBardVolume( value );
				}
			break;
		
		case CommandDefinition::PrefNewLogEveryJoin:
			bGotParam = ResolveBoolean( &word, &theBool, true );
			if ( bGotParam )
				gPrefsData.pdNewLogEveryJoin = theBool;
			break;
		
		case CommandDefinition::PrefNoMovieTextLogs:
			bGotParam = ResolveBoolean( &word, &theBool, true );
			if ( bGotParam )
				gPrefsData.pdNoMovieTextLogs = not theBool;
			break;
		
#endif	// COMMAND_MANY_PREFS
		}
	
	// give help message if we couldn't resolve the supplied pref value
	if ( not bGotParam )
		HandleHelpCommand( def );
}


/*
**	ClientCommand::HandleMoveCommand()
**	handle movements
**	'move <dir> walk | run'
**	'move stop'
*/
void
HandleMoveCommand( int cmdID, SafeString * cmdStr )
{
	// Get the next word
	SafeString word;
	GetWord( cmdStr, &word );
	
	// Resolve the third word - the speed
	const CommandDefinition * def = gMoveSpeedCommandDefs;
	int speed = CommandDefinition::ResolveCommand( &def, &word );
	
	MovePlayerToward( cmdID - CommandDefinition::Move, speed, false );
}


#if 0	// disabled: these are now server-side commands
// ClientCommand::HandleEquipCommand
//
// Equip an item, different from other HandleCommands,
// it takes the whole cmdStr as a parameter
// instead of just the parameter part
int
HandleEquipCommand( SafeString * /* cmdStr */ )
{
	return kDelayed;
}


// ClientCommand::HandleUnequipCommand
//
// Unequip an item
int
HandleUnequipCommand( int /* cmdID */ )
{
	return noErr;
}
#endif  // 0


/*
**	ClientCommand::HandleSelectItemCommand()
**
**	select or de-select an inventory item in the list
*/
void
HandleSelectItemCommand( SafeString * cmdStr )
{
	SafeString word, arg;
	GetWord( cmdStr, &word );
	
	arg.Set( word.Get() );
	
	DTSError err = noErr;
	SafeString errMsg;
	
	// check for next, previous
	const CommandDefinition * def = gSelectCommandDefs;
	int cmdID = CommandDefinition::ResolveCommand( &def, &arg );
	
	ulong delta = 0;
	switch ( cmdID )
		{
		case CommandDefinition::SelectNext:		delta = +1;			break;
		case CommandDefinition::SelectPrevious:	delta = -1; 		break;
		case CommandDefinition::SelectFirst:	delta = 0x7FFFFFFF; break;
		case CommandDefinition::SelectLast:		delta = 0x80000001;	break;
		}
	
	if ( delta )
		{
		// do delta
		
		// ... or, for now, cheat
			/* "* HandleSelectItemCommand: relative selections not yet implemented." */
		errMsg.Format(  _(TXTCL_CMD_ERR_NORELATIVESELECTIONS) );
		ShowInfoText( errMsg.Get() );
		return;
		}
	
	// if no arg, deselect
	if ( word.Size() <= 1 )
		err = SelectNamedItem( "" );
	else
	// try to find full name
	if ( ResolveItemName( &word, true ) )
		err = SelectNamedItem( word.Get() );
	
#if 0
	// debugging
	errMsg.Format( "* SelectNamedItem: %d", int(err) );
	ShowInfoText( errMsg.Get() );
#endif
}


/*
**	ClientCommand::HandleMovieCommand()
**
**	start or stop recording
*/
void
HandleMovieCommand( SafeString * cmdStr )
{
	SafeString word;
	GetWord( cmdStr, &word );
	
	bool bStartRecording = false;
	const char * errMsg = 0;
	
	if ( ResolveBoolean( &word, &bStartRecording, true ) )
		{
		// start or stop
		if ( bStartRecording )
			{
			// requested to start recording
			if ( CCLMovie::IsRecording() )
				{
					/* "* A movie is already being recorded." */ 
				errMsg = _(TXTCL_CMD_ERR_MOVIEALREADYRECORDING);
				}
			else
			if ( CCLMovie::IsReading() )
				{
					/* "* Cannot record a movie while playing one." */
				errMsg = _(TXTCL_CMD_ERR_NORECORDWHILEPLAYINGONE);
				}
			else
				{
				gDTSMenu->DoCmd( mRecordMovie );	// toggles -> on
					/* "* Recording started." */
				errMsg = _(TXTCL_CMD_ERR_RECORDINGSTARTED);
				}
			}
		else
			{
			// requested to stop recording
			if ( CCLMovie::IsRecording() )
				{
				gDTSMenu->DoCmd( mRecordMovie );	// toggles -> on
					/* "* Recording stopped." */
				errMsg = _(TXTCL_CMD_ERR_RECORDINGSTOPPED);
				}
			else
				{
					/* "* No movie is being recorded." */
				errMsg = _(TXTCL_CMD_ERR_NOMOVIEISRECORDED);
				}
			}
		
		// feedback
		if ( errMsg )
			ShowInfoText( errMsg );
		}
}


// ClientCommand::HandleLoggedServerCommand
//
// Print out something that resembles what they did, even though we don't know the outcome
void
HandleLoggedServerCommand( int cmdID, SafeString * cmdStr )
{
	SafeString logged;
	
	// Get rid of that darn \r, the last character
	// (if it's there -- it only appears to arise from macros)
	size_t len = strlen( cmdStr->Get() );
	if ( len
	&&	 '\r' == cmdStr->Get()[ len - 1 ] )
		{
		cmdStr->Get()[ len - 1 ] = '\0';
		}
	
	// Don't log null strings...
	if ( not *cmdStr->Get() )
		return;
	
	// setup a self-text style record
	CLStyleRecord style;
	SetUpStyle( kMsgMySpeech, &style );
	
	switch ( cmdID )
		{
		case CommandDefinition::Pray:
				/* "You pray, \"%s\"" */
			logged.Format( _(TXTCL_CMD_LOG_PRAY), cmdStr->Get() );
			AppendTextWindow( logged.Get(), &style );
			break;
		
		case CommandDefinition::Bug:
				/* "You report, \"%s\"" */
			logged.Format( _(TXTCL_CMD_LOG_REPORT), cmdStr->Get() );
			AppendTextWindow( logged.Get(), &style );
			break;
		
#ifdef DEBUG_VERSION
		case CommandDefinition::Use:
			logged.Format( "\\USE %s", cmdStr->Get() );
			AppendTextWindow( logged.Get() );
			break;
		
		case CommandDefinition::UseItem:
			logged.Format( "\\USEITEM %s", cmdStr->Get() );
			AppendTextWindow( logged.Get() );
			break;
		
		case CommandDefinition::Message:
			logged.Format( "\\MESSAGE %s", cmdStr->Get() );
			AppendTextWindow( logged.Get() );
			break;
#endif	// DEBUG_VERSION
		
// That's all the ones I can think of that need this...
		}
}


// ClientCommand::HandleHelp
//
// Lists all of the commands the client supports
void
HandleHelp( SafeString * cmdStr )
{
	SafeString word;
	GetWord( cmdStr, &word );
	GetWord( cmdStr, &word );
	
	if  ( not *word.Get() )
		{
		// List the commands
		SafeString list;
		list.Set( /* "Client commands:" */ _(TXTCL_CMD_CLIENT_COMMANDS) );
		const CommandDefinition * def = gCommandDefs;
		int ii = 0;
		while ( def[ ii ].cdKeyword )
			{
			if ( 0 == ii )
				list.Format( " \\%s",  def[ ii ].cdKeyword );
			else
				list.Format( ", \\%s", def[ ii ].cdKeyword );
			++ii;
			}
		list.Append( "." );
		ShowInfoText( list.Get() );
		}
}


// ClientCommand::HandleHelpCommand
//
// Writes a command's help string and lists any subcommands
void
HandleHelpCommand( const CommandDefinition * def )
{
	SafeString list;
	list.Set( def->cdHelp );
	
	def = def->cdSubCommands;
	if ( def )
		{
			/* "  Subcommands:" */
		list.Append( _(TXTCL_CMD_SUBCOMMANDS));
		int ii = 0;
		while ( def[ii].cdKeyword )
			{
			if ( 0 == ii )
				list.Format( " %s",  def[ ii ].cdKeyword );
			else
				list.Format( ", %s", def[ ii ].cdKeyword );
			++ii;
			}
		list.Append( "." );
		}
	ShowInfoText( list.Get() );
}

} // namespace ClientCommand


/*
**	StartNameComparisonFixed()
**
**	just like NameComparison, but assumes name1 is already compacted
**	saves some time
*/
int
StartNameComparisonFixed( const char * name1, const char * name2 )
{
	char cname2[ kMaxNameLen ];
	CompactName( name2, cname2, kMaxNameLen );
	return strncmp( name1, cname2, strlen(name1) );
}


#if 0  // unused
/*
**	WithinNameComparisonFixed()
**
**	as above, but looks for partial matches
**	(not currently used, however)
*/
int
WithinNameComparisonFixed( const char * name1, const char * name2 )
{
	char cname2[ kMaxNameLen ];
	CompactName( name2, cname2, kMaxNameLen );
	return (int) strstr( cname2, name1 ) ;
}
#endif  // 0

