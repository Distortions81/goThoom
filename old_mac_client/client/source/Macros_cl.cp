/*
**	Macros_cl.cp		ClanLord
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

#include "Macros_cl.h"
#include "MacroDefs_cl.h"
#include "SendText_cl.h"
#include "TextCmdList_cl.h"


/*
**	Internal Routines
*/
static CExecutingMacro * StartMacroExecution( CTriggerMacro * macro );
static void Continue1MacroExecution( CExecutingMacro * &on );
static void	FinishMacroExecution();
#ifdef MULTILINGUAL
static bool	CompareStrVariablewithShortTag( const char * value1, const char * value2 );
#endif // MULTILINGUAL

//
// Global linked lists
//
static CMacro *				gRootExpressionMacro;
static CMacro *				gRootReplacementMacro;
static CMacro *				gRootKeyMacro;
static CMacro *				gRootIncludeFileMacro;

static CMacro *				gRootVariableMacro;
static CMacro *				gRootFunctionMacro;
static CMacro *				gRootClickMacro;

static CExecutingMacro *	gExecutingMacro;


//
//	Class variables
//
char	CMacro::sTextWinLine[ kPlayerInputStrLen ];
bool	CExecutingMacro::sExInterrupted;
uint	CExecutingMacro::sNumGotos;


// symbolic names for various variables
#define	kVar_Click_name		"@click.name"
#define	kVar_Click_sname	"@click.simple_name"
#define	kVar_Click_Button	"@click.button"
#define	kVar_Click_Chord	"@click.chord"

#define kVar_Env_Echo		"@env.echo"
#define	kVar_Env_Debug		"@env.debug"
#define kVar_Env_KeyIntr	"@env.key_interrupts"
#define kVar_Env_ClickIntr	"@env.click_interrupts"
#define kVar_Env_Unfriendly	"@env.unfriendly"
#define kVar_Env_TextLog	"@env.textlog"

#define kVar_My_Name		"@my.name"
#define	kVar_My_Right		"@my.right_item"
#define kVar_My_Left		"@my.left_item"
#define kVar_Sel_Name		"@selplayer.name"
#define kVar_Sel_SName		"@selplayer.simple_name"

#define kVar_Text			"@text"
#define kVar_TextSel		"@textsel"
#define kVar_Intr_Key		"@interruptkey"
#define	kVar_Text_NumWords	"@text.num_words"

#define kMod_No_Repeat		"no-repeat"
#define kMacro_True			"true"
#define kEnd_Random			"end random"

// this is stupid and pointless but it seems to prevent
// the Xcode 3 syntax colorizer from going bananas.
struct CMacroKind;


//
// Command lookup tables
//
const CMacroKind gMacroVariableKinds[] =
{
	{ "@env",			CMacroKind::vEnv },			// Controlling macro execution
	{ "@my",			CMacroKind::vMy },			// Current player
	{ "@selplayer",		CMacroKind::vSelPlayer },	// Selected Player
	{ "@click",			CMacroKind::vClick },		// Clicked on player
	{ "@random",		CMacroKind::vRandom },
	{ nullptr,			0 }
};

// env
const CMacroKind gEnvVariables[] =
{
	{ "key_interrupts",		CMacroKind::vKeyInterrupts },
	{ "click_interrupts",	CMacroKind::vClickInterrupts },
	{ "debug",				CMacroKind::vDebug },
	{ "echo",				CMacroKind::vEcho },
	{ "unfriendly",			CMacroKind::vUnfriendly },
	{ "textlog",			CMacroKind::vTextLog },			// latest line sent to text log
	{ nullptr,				0 }
};

// any variable
const CMacroKind gTextVariables[] =
{
	{ "word[",				CMacroKind::vWord },
	{ "letter[",			CMacroKind::vLetter },
	{ "num_words",			CMacroKind::vNumWords },
	{ "num_letters",		CMacroKind::vNumLetters },
	{ nullptr,				0 }
};

// my and selplayer
const CMacroKind gPlayerVariables[] =
{
	{ "name",				CMacroKind::vRealName },
	{ "simple_name",		CMacroKind::vSimpleName },
	{ "left_item",			CMacroKind::vLeftItem },
	{ "right_item",			CMacroKind::vRightItem },
	{ "race",				CMacroKind::vRace },
	{ "gender",				CMacroKind::vGender },
	{ "health",				CMacroKind::vHealth },
	{ "balance",			CMacroKind::vBalance },
	{ "magic",				CMacroKind::vMagic },
	{ "shares_in",			CMacroKind::vSharesIn },
	{ "shares_out",			CMacroKind::vSharesOut },
	{ "forehead_item",		CMacroKind::vForehead },
	{ "neck_item",			CMacroKind::vNeck },
	{ "shoulders_item",		CMacroKind::vShoulders },
	{ "arms_item",			CMacroKind::vArms },
	{ "gloves_item",		CMacroKind::vGloves },
	{ "finger_item",		CMacroKind::vFinger },
	{ "coat_item",			CMacroKind::vCoat },
	{ "cloak_item",			CMacroKind::vCloak },
	{ "torso_item",			CMacroKind::vTorso },
	{ "waist_item",			CMacroKind::vWaist },
	{ "legs_item",			CMacroKind::vLegs },
	{ "feet_item",			CMacroKind::vFeet },
	{ "hands_item",			CMacroKind::vBothHands },
	{ "head_item",			CMacroKind::vHead },
	{ "selected_item",		CMacroKind::vSelectedItem },
	{ nullptr,				0 }
};

const CMacroKind gGetMacroVariables[] =
{
	{ "@name",				CMacroKind::vName },
	{ "@splayer",			CMacroKind::vSimplePlayerName },
	{ "@rplayer",			CMacroKind::vplayerName },
	{ "@rhanditem",			CMacroKind::vRightHandName },
	{ "@lhanditem",			CMacroKind::vLeftHandName },
	{ nullptr,				0 }
};

const CMacroKind gGetObsoleteMacroVariables[] =
{
	{ "@name",				CMacroKind::vName },
	{ "@splayer",			CMacroKind::vSimplePlayerName },
	{ "@rplayer",			CMacroKind::vplayerName },
	{ "@rhanditem",			CMacroKind::vRightHandName },
	{ "@lhanditem",			CMacroKind::vLeftHandName },
	{ "@echo",				CMacroKind::vOldEcho },
	{ "@debug",				CMacroKind::vOldDebug },
	{ "@interruptclick",	CMacroKind::vOldInterruptClick },
	{ "@interruptkey",		CMacroKind::vOldInterruptKey },
	{ "@clicksplayer",		CMacroKind::vOldClickSPlayer },
	{ "@clickrplayer",		CMacroKind::vOldClickRPlayer },
	{ "@wordcount",			CMacroKind::vOldWordCount },
	{ nullptr,				0 }
};

const CMacroKind gMacroAttributeKinds[] =
{
	{ "$ignore_case",		CMacroKind::aIgnoreCase },
	{ "$any_click",			CMacroKind::aAnyClick },
	{ "$no_override",		CMacroKind::aNoOverride },
	{ nullptr,				0 }
};

const CMacroKind gMacroKinds[] =
{
	{ "\"",					CMacroKind::mExpression },
	{ "'", 					CMacroKind::mReplacement },
	{ "set",				CMacroKind::mVariable },
	{ "include",			CMacroKind::mIncludeFile },
	{ nullptr,				0 }
};

const CMacroKind gMacroCmdKinds[] =
{
	{ "pause",				CMacroKind::cPause },
	{ "move",				CMacroKind::cMove },
	
	{ "set",				CMacroKind::cSetVariable },
	{ "setglobal",			CMacroKind::cSetGlobalVariable },
	{ "call",				CMacroKind::cCallFunction },
	
	{ "end",				CMacroKind::cEnd },
	
	{ "if",					CMacroKind::cIf },
	{ "else",				CMacroKind::cElse },
	
	{ "random",				CMacroKind::cRandom },
	{ "or",					CMacroKind::cOr },
	
	{ "label",				CMacroKind::cLabel },
	{ "goto",				CMacroKind::cGoto },
	
	{ "message",			CMacroKind::cMessage },
	
	{ "ignore_case", 		CMacroKind::cNotCaseSensitive },
	
	{ nullptr,				0 }
};

const CMacroKind gMacroControlKinds[] =
{
	{ "{",					CMacroKind::cStart },
	{ "}",					CMacroKind::cFinish },
	{ nullptr,				0 }
};


//
// Key Lookup tables
//
struct KeyNameMap
{
	const char *	knmName;
	int				knmKey;
};

const KeyNameMap gKeyNameMap[] =
{
	{ "escape",			kEscapeKey },
	{ "f1",				kF1Key },
	{ "f2",				kF2Key },
	{ "f3",				kF3Key },
	{ "f4",				kF4Key },
	{ "f5",				kF5Key },
	{ "f6",				kF6Key },
	{ "f7",				kF7Key },
	{ "f8",				kF8Key },
	{ "f9",				kF9Key },
	{ "f10",			kF10Key },
	{ "f11",			kF11Key },
	{ "f12",			kF12Key },
	{ "f13",			kF13Key },
	{ "f14",			kF14Key },
	{ "f15",			kF15Key },
	{ "f16",			kF16Key },
	{ "minus",			'-' },
	{ "delete",			kDeleteKey },
	{ "tab",			'\t' },
	{ "return",			kReturnKey },
	{ "space",			' ' },
	{ "help",			kHelpKey },
	{ "home",			kHomeKey },
	{ "pageup",			kPageUpKey },
	{ "del",			kRightDeleteKey },
	{ "end",			kEndKey },
	{ "pagedown",		kPageDownKey },
	{ "up",				kUpArrowKey },
	{ "down",			kDownArrowKey },
	{ "left",			kLeftArrowKey },
	{ "right",			kRightArrowKey },
	{ "clear",			kEscapeKey },
	{ "enter",			kEnterKey },
	{ "click",			kKeyClick },
	{ "click2",			kKeyClick2 },
	{ "right-click",	kKeyClick2 },	// synonym for click2
	{ "click3",			kKeyClick3 },
	{ "click4",			kKeyClick4 },
	{ "click5",			kKeyClick5 },
	{ "click6",			kKeyClick6 },
	{ "click7",			kKeyClick7 },
	{ "click8",			kKeyClick8 },
	{ "wheelup",		kKeyWheelUp },
	{ "wheeldown",		kKeyWheelDown },
	{ "wheelleft",		kKeyWheelLeft },
	{ "wheelright",		kKeyWheelRight },
	{ nullptr,			0 }
};

const KeyNameMap gModNameMap[] =
{
	{ "command",		kKeyModMenu },
	{ "control",		kKeyModControl },
	{ "numpad",			kKeyModNumpad },
	{ "option",			kKeyModOption },
	{ "shift",			kKeyModShift },
	{ nullptr,			0 }
};


//
// File headers
//	consider putting these into text files in the app's Resources folder
//
const char * const macroPlayerFile =
	"//	This macro file will be loaded for the player it is named after. \n\n"
	"include \"Default\"\n\n";

const char * const macroDefaultFile =
	"// This macro file is included for every character by default.\n"
	"// Use it for general macros that you want available to all characters.\n"
	"//\n\n"
	
	"\"??\"    \"/help \"    @text \"\\r\"\n"
	"\"aa\"    \"/action \"  @text \"\\r\"\n"
	"\"gg\"    \"/give \"    @text \"\\r\"\n"
	"\"ii\"    \"/info \"    @text \"\\r\"\n"
	"\"kk\"    \"/karma \"   @text \"\\r\"\n"
	"\"mm\"    \"/money\\r\"\n"
	"\"nn\"    \"/news\\r\"\n"
	"\"pp\"    \"/ponder \"  @text \"\\r\"\n"
	"\"sh\"    \"/share \"   @text \"\\r\"\n"
	"\"sl\"    \"/sleep\\r\"\n"
	"\"t\"     \"/think \"   @text \"\\r\"\n"
	"\"tt\"    \"/thinkto \" @text \"\\r\"\n"
	"\"th\"    \"/thank \"   @text \"\\r\"\n"
	"\"ui\"    \"/useitem \" @text \"\\r\"\n"
	"\"uu\"    \"/use \"     @text \"\\r\"\n"
	"\"un\"    \"/unshare \" @text \"\\r\"\n"
	"\"w\"     \"/who \"     @text \"\\r\"\n"
	"\"wh\"    \"/whisper \" @text \"\\r\"\n"
	"\"yy\"    \"/yell \"    @text \"\\r\"\n"
	"\n";


//
// String Utility functions
//

/*
**	breakstricmp()
**	Compares two strings, case-insensitively, stopping on breaks only
*/
static int
breakstricmp( const char * s, const char * p, const char * breaks )
{
	int ret = 0;
	while ( (*p || *s) && 0 == ret )
		{
		if ( strchr( breaks, * (const uchar *) s ) && *s )
			return ret;
		
		ret = toupper(* (const uchar *) s) - toupper(* (const uchar *) p);
		++s;
		++p;
		}
	return ret;
}


/*
**	beginistrcmp()
**	Compares two strings stopping on breaks and nulls
*/
static int
beginstricmp( const char * s, const char * p )
{
	int ret = 0;
	while ( *s && *p && 0 == ret )
		{
		ret = toupper(* (const uchar *) s) - toupper(* (const uchar *) p);
		++s;
		++p;
		}
	return ret;
}


/*
**	StripBeforePeriod()
**	trim away everything up to and including the first period
*/
static void
StripBeforePeriod( char * str )
{
	// Find a period
	const char * p = strchr( str, '.' );
	char * dest = str;
	
	// Set the string to "" if we didn't find one
	if ( not p )
		{
		*dest = '\0';
		return;
		}
	
	// Copy everything after the period if we did
	++p;
	while ( *p )
		{
		*dest++ = * (const uchar *) p++;
		}
	*dest = '\0';
}


/*
**	RemoveQuotes()
**	removes quotes (either " or ' but not both) from a string if they
**	entirely enclose it.
*/
static bool
RemoveQuotes( SafeString * dest, const char * expr )
{
	size_t l = strlen( expr );
	if ( l < 2 )
		return false;
	
	if ( ( '"'  == expr[0] && '"'  == expr[l - 1] )
	||	 ( '\'' == expr[0] && '\'' == expr[l - 1] ) )
		{
		dest->Set( expr + 1 );			// omit leading quote
	 	dest->Get()[ l - 2 ] = '\0';	// zap trailing quote
		return true;
		}
	
	return false;
}


/*
**	CopyExpression()
**	copy an expression by evaluating it in small stages:
**	first try to remove quotes,
**	otherwise look up a variable's value
**	if all else fails, treat it as a string literal
*/
static void
CopyExpression( SafeString * dest, const char * expr, CMacro * root )
{
	const char * copy = expr;
	
	// Remove surrounding quotes
	if ( RemoveQuotes( dest, expr ) )
		return;
	
	// This might be a variable; get its true value
	if ( CVariableMacro::GetVariable( expr, dest, root ) )
		return;
	
	dest->Set( copy );
}


/*
**	StringToNum()
**	Convert a string to a number
**	returns true if the string was a valid number
*/
static bool
StringToNum( const char * str, int& num )
{
	int tempnum;
	if ( 1 != sscanf( str, "%d", &tempnum ) )
		return false;
	
	num = tempnum;
	return true;
}


/*
**	GetKeyByName()
**	Returns the key and modifiers for a named key
*/
static bool
GetKeyByName( const char * inKey, int * outKey, uint * outModifiers )
{
	*outModifiers = 0;
	*outKey = 0;
	
	// Get modifier names & values
	for (;;)
		{
		const char * const ptr = strchr( inKey, '-' );
		if ( not ptr )
			break;
		
		if ( ptr != inKey )		// attempt to handle '--' with a modicum of dignity
			{
			char modName[ 64 ];
			BufferToStringSafe( modName, sizeof modName, inKey, ptr - inKey );
			for ( const KeyNameMap * map = gModNameMap; map->knmName; ++map )
				{
				if ( 0 == strcasecmp( modName, map->knmName ) )
					{
					*outModifiers |= static_cast<uint>( map->knmKey );
					break;
					}
				}
			}
		inKey = ptr + 1;
		}
	
	// get the key
	if ( 0 == inKey[1] )
		{
		// simple key
		*outKey = * reinterpret_cast<const uchar *>( inKey );
		}
	else
		{
		// named key
		for ( const KeyNameMap * map = gKeyNameMap; map->knmName; ++map )
			{
			if ( 0 == strcasecmp( inKey, map->knmName ) )
				{
				*outKey = map->knmKey;
				break;
				}
			}
		}
	return 0 != *outKey;
}


/*
**	RealToSimplePlayerName()
**	convert a player name to non-ambiguous form
**	that doesn't require quoting if sent to server
**	Very similar to CompactName(), except it works in-place and doesn't upper-case
*/
static void
RealToSimplePlayerName( char * name )
{
	char * d = name;
	const char * s = name;
	
	while ( *s )
		{
		if ( isalnum( * (const uchar *) s ) )
			*d++ = * (const uchar *) s;
		++s;
		}
	*d = '\0';
}

#pragma mark -


/*
**	CreateMissingMacroFile()
**
**	Create a file in the current directory if it isn't there
*/
static void
CreateMissingMacroFile( DTSFileSpec * theFileSpec, const char * fileData, size_t len = 0 )
{
	FILE * updateFile = theFileSpec->fopen( "r" );
	size_t l = len ? len : strlen( fileData );
	
	// Only overwrite if the file isn't found
	if ( not updateFile )
		{
		updateFile = theFileSpec->fopen( "w" );
		
		if ( updateFile )
			{
			const char * p = fileData;
			
			// Get rid of those nasty \r characters
			for ( size_t ii = 0;  ii < l;  ++ii )
				{
				if ( '\r' == *p )
					fputc( '\n', updateFile );
				else
					fputc( *p, updateFile );
				++p;
				}
			}
		else
			CMacro::ShowMacroInfoText( _(TXTCL_MACROS_ERRORCREATINGFILE),
						theFileSpec->GetFileName() );
			/* "Error creating file \"%s\".", */
#if 0
		char infoText[ 256 ];
		snprintf( infoText, sizeof infoText, BULLET "New macro file \"%s\" created.",
			theFileSpec->GetFileName() );
		ShowInfoText( infoText );
#endif
		}
	
	if ( updateFile )
		fclose( updateFile );
}


/*
**	InitMacros()
**
**	initialize macros
*/
DTSError
InitMacros()
{
	DTSError result;
	
	// Start from a blank slate
	KillMacros();
	
#if USE_MACRO_FOLDER
	
	char currentPlayer[ kMaxNameLen ];
	strcpy( currentPlayer, gPrefsData.pdCharName );
	
	// if currentPlayer is blank, do nothing
	if ( 0 == currentPlayer[0] )
		return -1;
	
	// make sure name portion has no ':' characters (as in friends folder),
	// to forestall invalid HFS filenames
	for ( char * p = currentPlayer;  *p;  ++p )
		if ( ':' == * (const uchar *) p )
			*p = '-';
	
	// Save the old directory
	DTSFileSpec mainDir;
	(void) mainDir.GetCurDir();
	
	// Create the macro directory or get a spec to an existing one
	DTSFileSpec macroDir = mainDir;
	macroDir.SetFileName( kCLMacros_FolderName );
	
	// create it if it doesn't exist
	DTSDate junkDate;
	result = macroDir.GetModifiedDate( &junkDate );
	if ( fnfErr == result )
		result = macroDir.CreateDir();
//	if ( noErr == result )
//		CMacro::ShowMacroInfoText( "%s folder created.", kCLMacros_FolderName );
	
	result = macroDir.SetDir();
	if ( result )
		{
		// Well, creating must have failed...
		CMacro::ShowMacroInfoText( /* "Error creating \"%s\" folder" */
					_(TXTCL_MACROS_ERRORCREATINGFOLDER), kCLMacros_FolderName );
		return result;
		}
	
	// get the resource that contains the current macro instructions text
	CFDataRef instructData = nullptr;
	const char * instructText = nullptr;
	ByteCount instructLen = 0;
	
	if ( CFURLRef u = CFBundleCopyResourceURL( CFBundleGetMainBundle(),
						CFSTR("MacroInstructions"), CFSTR("txt"), CFSTR("") ) )
		{
		SInt32 err = noErr;
		if ( CFURLCreateDataAndPropertiesFromResource( kCFAllocatorDefault, u,
				&instructData, nullptr, nullptr, &err ) )
			{
			instructText = reinterpret_cast<const char *>( CFDataGetBytePtr( instructData ) );
			instructLen = CFDataGetLength( instructData );
			}
		CFRelease( u );
		}
	
	// Do some maintenance on the Macro Folder in case some of its files have been renamed,
	// are missing, or need to be updated.
	
	DTSFileSpec macroFile = macroDir;
	if ( instructText )
		{
		macroFile.SetFileName( kCLMacros_InstructionFileName );
		
		// 	Check to see if the first line of the instruction file has changed
		//  Erase the file if it has
		FILE * updateFile = macroFile.fopen( "r" );
		
		if ( updateFile )
			{
			char line[ 1024 ];
			
			fgets( line, sizeof line - 1, updateFile );
			fclose( updateFile );
			
			// find first lines of both files
			ptrdiff_t len1 = strpbrk( instructText, "\n\r" ) - instructText;
			ptrdiff_t len2 = strpbrk( line, "\n\r" ) - line;
			
			// are they different?
			if ( len1 != len2
			|| 	 0 != strncmp( line, instructText, (len1 < len2) ? len1 : len2 ) )
				{
				result = macroFile.Delete();
				if ( noErr == result )
					{
					CMacro::ShowMacroInfoText( /* "\"%s\" updated." */
							_(TXTCL_MACROS_INFO_UPDATED), kCLMacros_InstructionFileName );
					}
				else
					CMacro::ShowMacroInfoText(
								_(TXTCL_MACROS_ERROR_UPDATE),
								/*	"Unable to update \"%s\". "
								"It might be open in another application." */
								kCLMacros_InstructionFileName );
				}
			}
		
		// recreate the macro instruction file if missing
		macroFile.SetFileName( kCLMacros_InstructionFileName );
		CreateMissingMacroFile( &macroFile, instructText, instructLen );
		}
	if ( instructData )
		CFRelease( instructData );
	
	// return to the macro folder, in case the instructions file was aliased elsewhere
	macroFile = macroDir;
	
	macroFile.SetFileName( kCLMacros_DefaultFileName );
	CreateMissingMacroFile( &macroFile, macroDefaultFile );
	
	// return to the macro folder, in case the "Default" file was aliased elsewhere
	macroFile = macroDir;
	
	// Write a new per-character macro file if this character doesn't have one
	macroFile.SetFileName( currentPlayer );
	CreateMissingMacroFile( &macroFile, macroPlayerFile );
	
	// Load the file with the current player's name
	macroFile.SetDirNoPath();
	CMacroParser theMacroParser;
	result = theMacroParser.ParseFile( currentPlayer );
	mainDir.SetDirNoPath();
	
#else
	result = ReadMacroFile( "clmacros" );
#endif // USE_MACRO_FOLDER
	
	
	//
	// this causes a macro called "@login" to be executed once per log-on
	// (also runs if you reload macros)
	//
	// note that the macro is a CFunctionMacro, because we're assuming you'd write it
	// like so:
	//
/*
@login
	{
	"/action yawns and stretches\r"
	pause 4
	"/pose sit\r"
	}
*/
	
	if ( CFunctionMacro * macro = CFunctionMacro::FindMacro( "@login", gRootFunctionMacro ) )
		{
		if ( CExecutingMacro * emacro = StartMacroExecution( macro ) )
			Continue1MacroExecution( emacro );
		}
	
	return result;
}


/*
**	KillMacros()
**	release all loaded macro data and halt execution
*/
void
KillMacros()
{
	CMacro::DeleteAll( &gRootExpressionMacro	);
	CMacro::DeleteAll( &gRootReplacementMacro	);
	CMacro::DeleteAll( &gRootKeyMacro			);
	CMacro::DeleteAll( &gRootClickMacro			);
	CMacro::DeleteAll( &gRootIncludeFileMacro	);
	CMacro::DeleteAll( &gRootFunctionMacro		);
	CMacro::DeleteAll( &gRootVariableMacro		);
	
	FinishMacroExecution();
	CMacro::sTextWinLine[0] = '\0';
}

#pragma mark -


/*
**	CMacro::CMacro()
**	macro base class constructor
*/
CMacro::CMacro( int kind, CMacro ** root ) :
	mKind( kind )
{
	// Add to the linked list
	InstallLast( *root );
}


/*
**	CMacro::DeleteAll()
**	delete a macro, its siblings, and all their descendants
*/
void CMacro::DeleteAll( CMacro ** root )
{
	// Delete any sub-lists
	for ( CMacro * macro = *root;  macro;  macro = macro->linkNext )
		{
		int kind = macro->mKind;
		if ( CMacroKind::mExpression	== kind
		|| 	 CMacroKind::mKey			== kind
		||	 CMacroKind::mFunction		== kind
		||	 CMacroKind::mReplacement	== kind )
			{
			CTriggerMacro * tMacro = static_cast<CTriggerMacro *>( macro );
			CMacro::DeleteLinkedList( tMacro->mTriggers );
			}
		}
	
	CMacro::DeleteLinkedList( *root );
}


/*
**	CMacro::FindMacro()
**	Find a general macro
*/
CMacro *
CMacro::FindMacro( int kind, CMacro * root )
{
	for ( CMacro * macro = root;  macro;  macro = macro->linkNext )
		{
		if ( macro->mKind == kind )
			return macro;
		}
	return nullptr;
}


/*
**	CMacro::VShowMacroInfoText()
**	display debugging information
*/
void
CMacro::VShowMacroInfoText( bool mustShow,
							CMacro * root,
							int currentLine,
							const char * fileName,
							const char * format,
							va_list params )
{
	// some messages aren't shown unless @env.debug is true
	if ( not mustShow
	&&	 not CVariableMacro::TestBool( kVar_Env_Debug, root ) )
		{
		return;
		}
	
	// create the message
	SafeString infoText;
	infoText.Set( BULLET " MACRO " );
	infoText.VFormat( format, params );
	
	// add file/line context data if we have it
	if ( fileName )
		{
		infoText.Format( "\rFILE \"%s\"", fileName );
		if ( currentLine )
			infoText.Format( " LINE %d", currentLine );
		}
	
	ShowInfoText( infoText.Get() );
}


/*
**	CMacro::ShowMacroInfoText()
**	same as above but called like printf()
*/
void
CMacro::ShowMacroInfoText( bool mustShow,
						   CMacro * root,
						   int currentLine,
						   const char * fileName,
						   const char * format, ... )
{
	va_list params;
	va_start( params, format );
	VShowMacroInfoText( mustShow, root, currentLine, fileName, format, params );
	va_end( params );
}


/*
**	CMacro::ShowMacroInfoText()
**	same as above, packaged for maximum convenience & minimal flexibility
*/
void
CMacro::ShowMacroInfoText( const char * format, ... )
{
	va_list params;
	va_start( params, format );
	
	// always shown; no root nor linenumber nor filename
	VShowMacroInfoText( true, nullptr, 0, nullptr, format, params );
	
	va_end( params );
}


/*
**	SetTextWinBuffer()
**
**	remember a line of text so @env.textLog can get to it
*/
void
CMacro::SetTextWinBuffer( const char * text )
{
	// maybe prefix it with a time stamp
	// (this should have been done by the caller, but that code [in TextWin_cl.cp]
	// doesn't lend itself to doing it clearly.)
	if ( gPrefsData.pdTimeStamp )
		{
		DTSDate date;
		date.Get();
		int hour = date.dateHour;
		char ampm = 'a';
		if ( hour >= 12 )
			ampm = 'p';
		hour = ( hour + 11 ) % 12 + 1;
		snprintf( sTextWinLine, sizeof sTextWinLine, "%d/%d/%2.2d %d:%2.2d:%2.2d%c %s",
			date.dateMonth,
			date.dateDay,
			date.dateYear % 100,
			hour,
			date.dateMinute,
			date.dateSecond,
			ampm,
			text );
		}
	else
		{
		// just copy it as is
		StringCopySafe( sTextWinLine, text, sizeof sTextWinLine );
		}
};

#pragma mark -


/*
**	CMacroKind::MacroIDToName()
**
**	perform a reverse symbol-table lookup
*/
void
CMacroKind::MacroIDToName( const CMacroKind * array, int m_id, SafeString * dest )
{
	for ( int ii = 0; array[ii].mStr; ++ii )
		{
		if ( m_id == array[ii].mID )
			{
			dest->Set( array[ii].mStr );
			return;
			}
		}
	
	dest->Clear();
}


/*
**	CMacroKind::MacroNameToID()
**	Find the ID of a string that matches str from the array
**	compares until a break is reached in str
*/
int
CMacroKind::MacroNameToID( const CMacroKind * const array,
						   const char * str,
						   const char * breaks /* ="" */ )
{
	for ( int ii = 0; array[ii].mStr; ++ii )
		{
		if ( 0 == breakstricmp( str, array[ii].mStr, breaks ) )
			return array[ii].mID;
		}
	return 0;
}


/*
**	CMacroKind::MacroNameBeginningToID()
**	lookup by name prefix
*/
int
CMacroKind::MacroNameBeginningToID( const CMacroKind * const array, const char * str )
{
	size_t ll = strlen( str );
	for ( int ii = 0; array[ii].mStr; ++ii )
		{
		if ( 0 == beginstricmp( str, array[ii].mStr )
		&&	 ll >= strlen( array[ii].mStr ) )
			{
			return array[ii].mID;
			}
		}
	return 0;
}

#pragma mark -

//
//	Expression Macros
//


/*
**	CExpressionMacro::CExpressionMacro()
**	constructor
*/
CExpressionMacro::CExpressionMacro( const char * expression, CMacro ** root ) :
	CTriggerMacro( CMacroKind::mExpression, root )
{
	mExpression.Set( expression );
	
	// abort if the SafeString failed to allocate memory
	// [historical relic from OS9]
	if ( not mExpression.Get() )
		{
		Remove( *root );
		delete this;
		}
}


/*
**	CExpressionMacro::FindMacro()
**	lookup by name of expression
*/
CExpressionMacro *
CExpressionMacro::FindMacro( const char * expression, CMacro * root )
{
	for ( CMacro * macro = root;  macro;  macro = macro->linkNext )
		{
		if ( CMacroKind::mExpression == macro->mKind )
			{
			CExpressionMacro * emacro = static_cast<CExpressionMacro *>( macro );
			
			// is this macro's name case-sensitive?
			if ( emacro->mAttributes & CMacroKind::aIgnoreCase )
				{
				if ( 0 == strcasecmp( expression, emacro->mExpression.Get() ) )
					return emacro;
				}
			else
				{
				if ( 0 == strcmp( expression, emacro->mExpression.Get() ) )
					return emacro;
				}
			}
		}
	return nullptr;
}

#pragma mark -


//
//	Function Macros
//


/*
**	CFunctionMacro::CFunctionMacro()
**	constructor
*/
CFunctionMacro::CFunctionMacro( const char * name, CMacro ** root ) :
	CTriggerMacro( CMacroKind::mFunction, root )
{
	mName.Set( name );
	
	// ran out of memory?
	if ( not mName.Get() )
		{
		Remove( *root );
		delete this;
		}
}


/*
**	CFunctionMacro::FindMacro()
**	lookup by name of function
*/
CFunctionMacro *
CFunctionMacro::FindMacro( const char * name, CMacro * root )
{
	for ( CMacro * macro = root;  macro;  macro = macro->linkNext )
		{
		if ( CMacroKind::mFunction == macro->mKind )
			{
			CFunctionMacro * fmacro = static_cast<CFunctionMacro *>( macro );
			
			if ( 0 == strcmp( name, fmacro->mName.Get() ) )
				return fmacro;
			}
		}
	return nullptr;
}

#pragma mark -


//
//	Replacement Macros
//


/*
**	CReplacementMacro::CReplacementMacro()
**	constructor
*/
CReplacementMacro::CReplacementMacro( const char * replace, CMacro ** root ) :
	CTriggerMacro( CMacroKind::mReplacement, root )
{
	mReplace.Set( replace );
	
	// check memory exhaustion
	if ( not mReplace.Get() )
		{
		Remove( *root );
		delete this;
		}
}


/*
**	CReplacementMacro::FindMacro()
**	lookup by name
*/
CReplacementMacro *
CReplacementMacro::FindMacro( const char * replace, CMacro * root )
{
	for ( CMacro * macro = root;  macro; macro = macro->linkNext )
		{
		if ( CMacroKind::mReplacement == macro->mKind )
			{
			CReplacementMacro * rmacro = static_cast<CReplacementMacro *>( macro );
			if ( rmacro->mAttributes & CMacroKind::aIgnoreCase )
				{
				if ( 0 == strcasecmp( replace, rmacro->mReplace.Get() ) )
					return rmacro;
				}
			else
				{
				if ( 0 == strcmp( replace, rmacro->mReplace.Get() ) )
					return rmacro;
				}
			}
		}
	return nullptr;
}

#pragma mark -


//
//	Key Macros
//


/*
**	CKeyMacro::FindMacro()
**	lookup by keycode and modifiers
*/
CKeyMacro *
CKeyMacro::FindMacro( int key, uint modifiers, CMacro * root )
{
	// ignore capslock
	modifiers &= ~kKeyModCapsLock;
	
	for ( CMacro * macro = root;  macro;  macro = macro->linkNext )
		{
		if ( macro->mKind == CMacroKind::mKey )
			{
			CKeyMacro * kmacro = static_cast<CKeyMacro *>( macro );
			
			// as explained somewhere in the macro documentation, this implementation
			// requires that if, for example, you want your macro to be triggered by
			// an option-e keystroke, you must name it "opt-<e-with-accent>".
			// Cf. the editor's MacroTool.cp for a different approach that would let
			// you name that macro simply as "opt-e".
			
			if ( kmacro->mKey == key
			&&	 kmacro->mModifiers == modifiers )
				{
				return kmacro;
				}
			}
		}
	return nullptr;
}

#pragma mark -


/*
**	CIncludeFileMacro::CIncludeFileMacro()
**	include-file constructor
*/
CIncludeFileMacro::CIncludeFileMacro( const char * name, CMacro ** root ) :
	CMacro( CMacroKind::mIncludeFile, root )
{
	mFileName.Set( name );
	
	// check memory
	if ( not mFileName.Get() )
		{
		Remove( *root );
		delete this;
		}
}


/*
**	CIncludeFileMacro::FindMacro()
**	lookup by name (of file)
*/
CIncludeFileMacro *
CIncludeFileMacro::FindMacro( const char * fileName, CMacro * root )
{
	for ( CMacro * macro = root;  macro;  macro = macro->linkNext )
		{
		if ( CMacroKind::mIncludeFile == macro->mKind )
			{
			CIncludeFileMacro * ifmacro = static_cast<CIncludeFileMacro *>( macro );
			
			// filenames are never case-sensitive
			if ( 0 == strcasecmp( fileName, ifmacro->mFileName.Get() ) )
				return ifmacro;
			}
		}
	return nullptr;
}

#pragma mark -

//
//	Variable Macros
//


/*
**	CVariableMacro constructor
*/
CVariableMacro::CVariableMacro( const char * name, CMacro ** root ) :
	CMacro( CMacroKind::mVariable, root )
{
	mName.Set( name );
	if ( not mName.Get() )
		{
		Remove( *root );
		delete this;
		}
}


/*
**	CVariableMacro::FindMacro()
**	name lookup
*/
CVariableMacro *
CVariableMacro::FindMacro( const char * name, CMacro * root )
{
	for ( CMacro * macro = root;  macro;  macro = macro->linkNext )
		{
		if ( CMacroKind::mVariable == macro->mKind )
			{
			CVariableMacro * vmacro = static_cast<CVariableMacro *>( macro );
			if ( 0 == strcmp( name, vmacro->mName.Get() ) )
				return vmacro;
			}
		}
	return nullptr;
}


/*
**	CVariableMacro::ArrayElementToVarName()
**	resolves index of var[ index ]
*/
void
CVariableMacro::ArrayElementToVarName( SafeString * name, CMacro * root )
{
	SafeString s;
	SafeString indexStr;
	
	s.Set( name->Get() );
	
	// locate the rightmost set of brackets
	char * found  	= strrchr( s.Get(), '[' );
	char * foundEnd = found ? strchr( found, ']' ) : nullptr;
	
	// Keep going while it has valid brackets
	// Must be matching and in the right order
	while ( found && foundEnd && foundEnd > found )
		{
		// Kill beginning and end braces
		char * p = found;
		*p++ = '\0';
		*foundEnd = '\0';
		
		// strip quotes or resolve variables from material inside brackets
		CopyExpression( &indexStr, p, root );
		
		// try to rewrite the array subscript as an integer literal in simplest form
		int index;
		if ( StringToNum( indexStr.Get(), index ) )
			{
			name->Clear();
			name->Format( "%s[%d]%s", s.Get(), index, foundEnd + 1 );
			}
		
		// Set found to the offset from where it started + 1
		++found;
		
		s.Set( name->Get() );
		
		// Find next brackets
		found = strrchr( found, '[' );
		foundEnd = found ? strchr( found, ']' ) : nullptr;
		}
}


/*
**	CVariableMacro::SetVariable()
**	set value of variable
*/
void
CVariableMacro::SetVariable( const char * name, const char * value,
	CMacro ** root, bool dontResolve )
{
	SafeString ssName;
	ssName.Set( name );
	
	// Handle obsolete references (the ones you can set)
	int m_id = CMacroKind::MacroNameToID( gGetObsoleteMacroVariables, ssName.Get() );
	switch ( m_id )
		{
		case CMacroKind::vOldEcho:
			ssName.Set( kVar_Env_Echo );
			break;
		
		case CMacroKind::vOldDebug:
			ssName.Set( kVar_Env_Debug );
			break;
		
		case CMacroKind::vOldInterruptClick:
			ssName.Set( kVar_Env_ClickIntr );
			break;
		
		case CMacroKind::vOldInterruptKey:
			ssName.Set( kVar_Env_KeyIntr );
			break;
		}
	
	// Handle arrays
	CVariableMacro::ArrayElementToVarName( &ssName, *root );
	
	// Set a user variable
	CVariableMacro * macro = CVariableMacro::FindMacro( ssName.Get(), *root );
	if ( not macro )
		macro = NEW_TAG("CVariableMacro") CVariableMacro( ssName.Get(), root );
	
	if ( macro )
		{
		if ( dontResolve )
			macro->mValue.Set( value );
		else
			CopyExpression( &macro->mValue, value, *root );
		}
}


/*
**	CVariableMacro::GetEnvVar()
**	get the value of an environment-related variable
**	return false if not successful
**	right now, only @env.textLog is readable
*/
bool
CVariableMacro::GetEnvVar( const char * name, SafeString * dst )
{
	int m_id = CMacroKind::MacroNameToID( gEnvVariables, name, "." );
	
	if ( CMacroKind::vTextLog == m_id )
		{
		dst->Set( sTextWinLine );
		return true;
		}
	
	return false;
}


/*
**	CVariableMacro::GetMyVar()
**	get the value of a "me"-related variable
*/
bool
CVariableMacro::GetMyVar( const char * name, SafeString * dst )
{
	bool result = false;
	
	int m_id = CMacroKind::MacroNameToID( gPlayerVariables, name, "." );
	char buff[ 256 ];
	
	switch ( m_id )
		{
		case CMacroKind::vRealName:
			if ( gThisPlayer )
				dst->Set( gThisPlayer->descName );
			result = true;
			break;
		
		case CMacroKind::vSimpleName:
			if ( gThisPlayer )
				{
				dst->Set( gThisPlayer->descName );
				RealToSimplePlayerName( dst->Get() );
				}
			result = true;
			break;
		
		case CMacroKind::vLeftItem:
		case CMacroKind::vRightItem:
			{
			int whichSlot = (CMacroKind::vRightItem == m_id) ?
						kItemSlotRightHand :
						kItemSlotLeftHand;
			GetInvItemNameBySlot( whichSlot, buff, sizeof buff );
			dst->Set( buff );
			result = true;
			} break;
		
		case CMacroKind::vRace:
			break;
		
		case CMacroKind::vGender:
			break;
		
		case CMacroKind::vHealth:
		//	if ( gFrame )
		//		dst->Format( "%d", gFrame->mHP );
			break;
		
		case CMacroKind::vBalance:
		//	if ( gFrame )
		//		dst->Format("%d", gFrame->mBalance );
			break;
		
		case CMacroKind::vMagic:
		//	if ( gFrame )
		//		dst->Format("%d", gFrame->mSP );
			break;
		
		case CMacroKind::vSharesIn:
		case CMacroKind::vSharesOut:
			ListShares( dst, CMacroKind::vSharesOut == m_id );
			result = true;
			break;
		
		case CMacroKind::vForehead ... CMacroKind::vHead:
			{
			// this exact ordering is important!
			// must match CMacroKind::vForehead..vHead
			static const uchar invSlots[] =
				{
				kItemSlotForehead,
				kItemSlotNeck,
				kItemSlotShoulder,
				kItemSlotArms,
				kItemSlotGloves,
				kItemSlotFinger,
				kItemSlotCoat,
				kItemSlotCloak,
				kItemSlotTorso,
				kItemSlotWaist,
				kItemSlotLegs,
				kItemSlotFeet,
//				kItemSlotRightHand,
//				kItemSlotLeftHand,
				kItemSlotBothHands,
				kItemSlotHead
				};
			
			int whichSlot = invSlots[ m_id - CMacroKind::vForehead ];
			GetInvItemNameBySlot( whichSlot, buff, sizeof buff );
			dst->Set( buff );
			result = true;
			} break;

		case CMacroKind::vSelectedItem:
			GetSelectedItemName( buff, sizeof buff );
			dst->Set( buff );
			result = true;
			break;
		}
	
	return result;
}


/*
**	CVariableMacro::GetSelPlayerVar()
**	get value of a player-related variable
*/
bool
CVariableMacro::GetSelPlayerVar( const char * name, SafeString * dst )
{
	int m_id = CMacroKind::MacroNameToID( gPlayerVariables, name, "." );
	switch ( m_id )
		{
		case CMacroKind::vRealName:
			dst->Set( gSelectedPlayerName );
			return true;
		
		case CMacroKind::vSimpleName:
			dst->Set( gSelectedPlayerName );
			RealToSimplePlayerName( dst->Get() );
			return true;
		
		// can't get these for non-self players
		// although most would be feasible, in principle, if you didn't
		// mind potentially stale data
/*
		case CMacroKind::vLeftItem:
		case CMacroKind::vRightItem:
		case CMacroKind::vRace:
		case CMacroKind::vGender:
		case CMacroKind::vHealth:
		case CMacroKind::vBalance:
		case CMacroKind::vMagic:
*/
		default:
			break;
		}
	return false;
}


/*
**	CVariableMacro::GetVariable()
**	top-level variable accessor
*/
bool
CVariableMacro::GetVariable( const char * name, SafeString * dst, CMacro * root )
{
	if ( not name || not *name )
		return false;
	
	bool set = false;
	dst->Clear();
	
	SafeString ssName;
	ssName.Set( name );
	
	// Convert obsolete variable names to new ones
	int m_id = CMacroKind::MacroNameToID( gGetObsoleteMacroVariables, ssName.Get() );
	switch ( m_id )
		{
		case CMacroKind::vName:
			ssName.Set( kVar_My_Name );
			break;
		
		case CMacroKind::vSimplePlayerName:
			ssName.Set( kVar_Sel_SName );
			break;
		
		case CMacroKind::vplayerName:
			ssName.Set( kVar_Sel_Name );
			break;
		
		case CMacroKind::vRightHandName:
			ssName.Set( kVar_My_Right );
			break;
		
		case CMacroKind::vLeftHandName:
			ssName.Set( kVar_My_Left );
			break;
		
		case CMacroKind::vOldEcho:
			ssName.Set( kVar_Env_Echo );
			break;
		
		case CMacroKind::vOldDebug:
			ssName.Set( kVar_Env_Debug );
			break;
		
		case CMacroKind::vOldInterruptClick:
			ssName.Set( kVar_Env_ClickIntr );
			break;
		
		case CMacroKind::vOldInterruptKey:
			ssName.Set( kVar_Env_KeyIntr );
			break;
		
		case CMacroKind::vOldClickSPlayer:
			ssName.Set( kVar_Click_sname );
			break;
		
		case CMacroKind::vOldClickRPlayer:
			ssName.Set( kVar_Click_name );
			break;
		
		case CMacroKind::vOldWordCount:
			ssName.Set( kVar_Text_NumWords );
			break;
		}
	
	// Catch the old word array
	if ( 0 == beginstricmp( "@word[", name ) )
		{
		ssName.Clear();
		ssName.Format( "@text.%s", name + 1 );
		}
	
	// Save the updated name
	SafeString saveName;
	saveName.Set( ssName.Get() );
	
	// Finally the real variable
	m_id = CMacroKind::MacroNameToID( gMacroVariableKinds, ssName.Get(), "." );
	StripBeforePeriod( ssName.Get() );
	
	switch ( m_id )
		{
		// @env
		case CMacroKind::vEnv:
			set = GetEnvVar( ssName.Get(), dst );
			break;
		
		// @my
		case CMacroKind::vMy:
			set = GetMyVar( ssName.Get(), dst );
			break;
		
		// @selplayer
		case CMacroKind::vSelPlayer:
			set = GetSelPlayerVar( ssName.Get(), dst );
			break;
		
		// @random
		case CMacroKind::vRandom:
			dst->Format( "%d", static_cast<int>( GetRandom( 10000 ) ) );
			set = true;
			break;
		}
	
	// Check to see if the variable has been assigned (even to nothing)
	if ( not set )
		{
		ssName.Set( saveName.Get() );
		CVariableMacro::ArrayElementToVarName( &ssName, root );
		
		// Look through user defined variables
		
		// Kill after a period; user variables can't have periods
		int period = 0;
		if ( char * p = strchr( ssName.Get(), '.' ) )
			{
			// If it was one of the defined variables, go to the second period
			if ( m_id )
				p = strchr( p + 1, '.' );
			
			if ( p )
				{
				*p = '\0';
				period = p - ssName.Get();
				}
			}
		
		// try local vars first
		CVariableMacro * macro = CVariableMacro::FindMacro( ssName.Get(), root );
		
		// if that fails, try global vars
		if ( not macro && root != gRootVariableMacro )
			macro = CVariableMacro::FindMacro( ssName.Get(), gRootVariableMacro );
		
		// found it?
		if ( macro )
			{
			set = true;
			dst->Set( macro->mValue.Get() );
			}
		
		// If there was a period, set ssName to what is after the dead spot
		if ( period )
			ssName.Set( ssName.Get() + period + 1 );
		}
	else
		{
		// was already set; deal with arrays
		CVariableMacro::ArrayElementToVarName( &ssName, root );
		}
	
	// Handle the word and letter trailers no matter what the variable
	if ( set )
		{
		while ( *ssName.Get() )
			{
			// look for "word", "letter", "num_words", "num_letters"
			m_id = CMacroKind::MacroNameBeginningToID( gTextVariables, ssName.Get() );

// !! CHECK THIS !!
//			if ( not m_id )
//				m_id = CMacroKind::MacroNameToID( gTextNumVariables, ssName.Get(), "." );
// !!!
			
			// Look for a number between brackets
			bool haveNumber = false;
			int num = 0;
			const char * period = strchr( ssName.Get(), '.' );
			char * openbracket  = strchr( ssName.Get(), '[' );
			char * closebracket = strchr( ssName.Get(), ']' );
			
			// we require both brackets, and either no periods at all, or else
			// that all periods be strictly to the right of the close bracket
			if ( closebracket && openbracket
			&&	 ( not period
			   ||  (  openbracket  < period
				   && closebracket < period
				   && openbracket  < closebracket )) )
				{
				++openbracket;
				closebracket = 0;
				haveNumber = StringToNum( openbracket, num );
				}
			
			StripBeforePeriod( ssName.Get() );
			
			/* static */ char param[ kPlayerInputStrLen ];		// for extracting words
			switch ( m_id )
				{
				// "foo.word[ n ]" -- extract n'th word
				case CMacroKind::vWord:
					{
					if ( not haveNumber )
						break;
					
					int count = -1;
					const char * cur = dst->Get();
					
					do
						{
						cur = CMacroParser::GetWord( cur, param, sizeof param, "" );
						++count;
						} while ( cur && *param && count < num );
					
					// destination gets the n'th word
					dst->Clear();
					if ( count <= num && *param )
						dst->Set( param );
					}
					break;
				
				// "foo.num_words" -- count words of foo
				case CMacroKind::vNumWords:
					{
					int count = 0;
					const char * cur = dst->Get();
					
					cur = CMacroParser::GetWord( cur, param, sizeof param, "" );
					while ( cur && *param )
						{
						++count;
						cur = CMacroParser::GetWord( cur, param, sizeof param, "" );
						}
					
					// destination gets the # of words
					dst->Clear();
					dst->Format( "%d", count );
					}
					break;
				
				// "foo.letter[ n ] -- extract n'th letter
				case CMacroKind::vLetter:
					{
					if ( not haveNumber )
						break;
					
					// destination gets the n'th character (if in range)
					int c = 0;
					if ( num < (int) strlen( dst->Get() )
					&&	 num >= 0 )
						{
						c = dst->Get()[ num ];
						}
					dst->Clear();
					if ( c )
						dst->Format( "%c", c );
					}
					break;
				
				// "foo.num_letters" -- count characters in foo
				case CMacroKind::vNumLetters:
					{
					int len = strlen( dst->Get() );
					dst->Clear();
					dst->Format( "%d", len );
					}
					break;
				}
			}
		}
	
	return set;
}


/*
**	CVariableMacro::TestBool()
**	compare a variable against constant "true"
*/
bool
CVariableMacro::TestBool( const char * name, CMacro * root )
{
	SafeString var;
	if ( GetVariable( name, &var, root ) )
		return 0 == strcasecmp( var.Get(), kMacro_True );
	
	return false;
}

#pragma mark -

//
//	Label Macros
//


/*
**	CLabelCmdMacro::CLabelCmdMacro()
**	label constructor
*/
CLabelCmdMacro::CLabelCmdMacro( const char * name, CMacro ** root ) :
	CMacro( CMacroKind::cLabel, root )
{
	mName.Set( name );
	
	// check memory
	if ( not mName.Get() )
		{
		Remove( *root );
		delete this;
		}
}


/*
**	CLabelCmdMacro::FindMacro()
**	lookup by name of label
*/
CLabelCmdMacro *
CLabelCmdMacro::FindMacro( const char * name, CMacro * root )
{
	for ( CMacro * macro = root;  macro;  macro = macro->linkNext )
		{
		if ( CMacroKind::cLabel == macro->mKind )
			{
			CLabelCmdMacro * lmacro = static_cast<CLabelCmdMacro *>( macro );
			
			// labels are case-sensitive
			if ( 0 == strcmp( name, lmacro->mName.Get() ) )
				return lmacro;
			}
		}
	
	return nullptr;
}

#pragma mark -

//
//	Parameter Macros
//


/*
**	CParameterCmdMacro::CParameterCmdMacro()
**	constructor for parameter macro
*/
CParameterCmdMacro::CParameterCmdMacro( const char * param, CMacro ** root ) :
	CMacro( CMacroKind::cParameter, root )
{
	mParam.Set( param );
	
	// check memory
	if ( not mParam.Get() )
		{
		Remove( *root );
		delete this;
		}
}

#pragma mark -


//
//	Macro Parser
//

/*
**	CMacroParser::CMacroParser()
**	parser constructor
*/
CMacroParser::CMacroParser() :
	cmdLevel( 0 ),
	currentLine( 0 ),
	lastMacro( nullptr )
{
}


/*
**	ReadMacroFile()
**	Reads in the specified macro file from the current directory
*/
DTSError
CMacroParser::ParseFile( const char * fname )
{
	DTSError result = noErr;
	
	// prepare a filespec for this file
	DTSFileSpec theFile;
	theFile.GetCurDir();
	theFile.SetFileName( fname );
	
	// setup the reading position markers
	fileName.Set( fname );
	
	FILE * stream = theFile.fopen( "r" );
	if ( not stream )
		{
		ParseMsg( true, /* "Error opening file." */ _(TXTCL_MACROS_ERROR_OPEN_FILE) );
		return noErr;
		}
	currentLine = 1;
	
	// prevent zillions of error messages if they try to read a UTF16 file
	// ... we might want to try to think about being smarter if we see a
	// unicode BOM, in either UTF8 (0xEF BB BF) or UTF16 (0xFF 0xFE) ...
	// ... and what about the byte-swapped variants of those? Ick!
	bool bShowedNullWarning = false;
	
	int fileFlavor = 0;		// 0 = unknown, '\n' = mac, '\r' = unix, anything else = ???
	if ( noErr == result )
		{
		// Read each line
		char line[ 5000 ];
		char * p = line - 1;
		
		while ( noErr == result )
			{
			const int ch = fgetc( stream );
			
			// end of file?
			if ( EOF == ch )
				{
				*++p = '\0';
				// deal with files that don't have a trailing CR
				if ( line != p )
					result = ParseLine( line );
				break;
				}
			else
			if ( 0 == ch )
				{
				// how'd a null get in here? squawk, and ignore it.
				if ( not bShowedNullWarning )
					{
					ParseMsg( true, _(TXTCL_MACROS_ERROR_CONTAINS_NULL)
/* 						"File contains a null character. (Did you accidentally save it "
						 "as UTF-16?)  I'm going to ignore it for now, "
						 " but you should fix the file." */ );
					bShowedNullWarning = true;
					}
				}
			else
			// end of line?
			if ( '\r' == ch
			||	 '\n' == ch )
				{
				if ( 0 == fileFlavor )
					fileFlavor = ch;
				else
				if ( ch != fileFlavor )
					{
					// try to do something not totally stupid with DOS files...
					// namely, if we're at the start of a line and see a 'foreign'
					// terminator, just ignore it: that should keep the line count right.
					// (we ignore the fact that strictly speaking DOS endings are 0d0a
					// not 0a0d -- or is it the other way?. Oh well. Who cares.)
					if ( p == line - 1 )
						continue;
					}
				
				// parse what we have, and prepare for next line
				*++p = '\0';
				p = line - 1;
				result = ParseLine( line );
				++currentLine;
				}
			else
			// C-style comment?
			if ( '/' == ch )
				{
				int ch2 = fgetc( stream );
				if ( '*' == ch2 )
					{
					IgnoreComment( stream );
					continue;
					}
				
				ungetc( ch2, stream );
				if ( p < line + sizeof line - 1 )
					*++p = ch;
				}
			else
				{
				// anything else, save in buffer. Ignore overflows.
				if ( p < line + sizeof line - 1 )
					*++p = ch;
				}
			}
		
		// proper nesting must be observed
		if ( cmdLevel > 0 && noErr == result )
			ParseMsg( true, /* "File is missing end brackets '}'." */
				_(TXTCL_MACROS_ERROR_MISSING_END_BRACKETS) );
		}
	
	if ( stream )
		fclose( stream );
	
	if ( noErr != result )
		{
		if ( memFullErr == result )
			ParseMsg( true, /* "Out of memory." */ _(TXTCL_MACROS_ERROR_OUTOFMEMORY) );
		else
		if ( -1 != result )
			ParseMsg( true, /* "Parsing Error." */ _(TXTCL_MACROS_ERROR_PARSINGERROR) );
		
		// Other files don't need to repeat saying this error
		result = -1;
		}
	
	return result;
}


/*
**	CMacroParser::IgnoreComment()
**
**	read and ignore a C-style comment
**	which extend to the very next occurrence of '*' and '/'
**	_without_ regard for quotes, backslashes, or any other consideration.
*/
void
CMacroParser::IgnoreComment( FILE * stream )
{
	bool bGotStar = false;		// was the last character an asterisk?
	for (;;)
		{
		int ch = fgetc( stream );
		
		// test for end of comment
		if ( '/' == ch && bGotStar )
			break;					// we're done
		
		if ( '*' == ch )
			{
			bGotStar = true;
			continue;
			}
		
		bGotStar = false;
		
		// line ending?
		if ( '\n' == ch || '\r' == ch )
			{
			// keep the line count correct.
			++currentLine;
			}
		else
		if ( EOF == ch )
			{
			// squawk & bail on premature EOF
				/* "Unterminated '/ *' comment at end of file." */
			ParseMsg( true, _(TXTCL_MACROS_ERROR_UNTERMINATEDCOMMENT) );
			break;
			}
		}
}


//
// try to build up a macro using this line
//
DTSError
CMacroParser::ParseLine( const char * theLine )
{
	DTSError err = noErr;
	char word[ kPlayerInputStrLen ];
	
	// grab the first word
	const char * line = NewWord( theLine, word );
	
	// This line is commented or blank
	if ( 0 == strncmp( word, "//", 2 )
	||	 '\0' == word[0] )
		{
		return noErr;
		}
	
	// Handle begin and end commands (curly braces)
	switch ( CMacroKind::MacroNameToID( gMacroControlKinds, word ) )
		{
		case CMacroKind::cStart:
			++cmdLevel;
			return noErr;
		
		case CMacroKind::cFinish:
			if ( --cmdLevel < 0 )
				{
				ParseMsg( true, /* "Unexpected closing brace encountered '}'" */
					_(TXTCL_MACROS_ERROR_UNEXPECTEDCLOSINGBRACE) );
				cmdLevel = 0;
				}
			
			// Allow a new macro to be loaded
			return noErr;
		}
	
	if ( 0 == cmdLevel )
		lastMacro = nullptr;
	
	// It is a new macro definition if the last thing loaded wasn't a macro
	if ( *word && not lastMacro && 0 == cmdLevel )
		{
		CTriggerMacro * macro = nullptr;
		err = NewMacro( macro, word, line );
		if ( err )
			return err;
		lastMacro = macro;
		}
	
	// Don't try to load commands if there is no macro to do it to.
	if ( not lastMacro )
		return noErr;
	
	// The first word defines the command
	if ( *word && not err )
		err = NewCommand( word, line );
	
	// All remaining words are parameters
	while ( *word && not err )
		err = NewParameter( word, line );
	
	return err;
}


/*
**	CMacroParser::NewWord()
**	read a new word
*/
const char *
CMacroParser::NewWord( const char * inLine, char * outDest )
{
	return GetWord( inLine, outDest, kPlayerInputStrLen, nullptr, nullptr,
				currentLine, fileName.Get() );
}


/*
**	CMacroParser::GetWord()
**	Get a word
**	Separators separate words, quotes make it load from quote to quote
**	Returns the line without the word it got
*/
const char *
CMacroParser::GetWord(
	const	char *	inLine,
			char *	outDest,
			size_t	inMaxLen,
	const	char *	inQuotes,
	const	char *	inSeparators,
			int		line,
	const	char *	file )
{
	const	uchar *	p		= reinterpret_cast<const uchar *>( inLine );
			uchar *	d		= reinterpret_cast<uchar *>( outDest );
	const	char *	quotes	= inQuotes ? inQuotes : "\"'";
	const	char *	wsep	= inSeparators ? inSeparators : kCLMacros_BreakString;
			char	quote	= 0;
			bool	done	= false;
	
	if ( not inLine )
		{
		*d = '\0';
		return nullptr;
		}
	
	// Skip initial spaces
	while ( strchr( kCLMacros_BreakString, *p ) && *p )
		++p;
	
	// See if the first character is a quote
	if ( strchr( quotes, *p ) && *p )
		{
		quote = *p;
		*d++ = *p++;
		}
	
	// Go until a matching quote or a break or the end
	while ( *p && (d - reinterpret_cast<uchar *>( outDest )) < (int) inMaxLen && not done )
		{
		if ( *p == quote )
			{
			quote = 0;
			*d++ = *p++;
			done = true;
			}
		else
			{
			switch ( *p )
				{
				// Handle escaped characters
				case '\\':
					++p;
					switch( *p )
						{
						// Carriage return
						case 'r':
							++p;
							*d++ = '\r';
							break;
						
						// Escaped characters
						case '"':
						case '\'':
						case '\\':
							*d++ = *p++;
							break;
						
						// Write out the slash and go on
						default:
							*d++ = '\\';
							break;
						}
					break;
				
				// handle comments
				case '/':
					if ( '/' == p[1]
					&&	 not quote )
						// found an unquoted comment
						done = true;
					else
						*d++ = *p++;
					break;
				
				// Copy unless we are done
				default:
					// Handle separators (when not in quotes), and carriage returns (always)
					if ( ( not quote && strchr( wsep, *p ) )
					||	 '\r' == *p
					||	 '\n' == *p )
						{
						done = true;
						}
					else
						*d++ = *p++;
				}
			}
		}
	
	// Catch strings that are too long
	if 	( d - reinterpret_cast<uchar *>( outDest ) >= int( inMaxLen ) )
		{
		*d = '\0';
		CMacro::ShowMacroInfoText( true, nullptr, line, file,
					/* "String is too long: %s" */
					_(TXTCL_MACROS_ERROR_STRINGTOOLONG), outDest );
		*outDest = '\0';
		return nullptr;
		}
	
	// There is no matching quote
	if ( quote )
		{
		*d = '\0';
		CMacro::ShowMacroInfoText( true, nullptr, line, file,
					/* "Matching %c not found: %s" */
					_(TXTCL_MACROS_ERROR_MATCHINGNOTFOUND), quote, outDest );
		
(void) 0;	// this NOP is here only to keep the Xcode3 syntax colorizer sane
		
		*outDest = '\0';
		return nullptr;
		}
	
	// Terminate the string
	*d = '\0';
	
	return reinterpret_cast<const char *>( p );
}


/*
**	CMacroParser::NewMacro()
**	read and define a new macro, which can be of type:
**		expression, replacement, key, or function
*/
DTSError
CMacroParser::NewMacro( CTriggerMacro *& macro, char * word, const char *& line )
{
	SafeString s;
	DTSError err = noErr;
	macro = nullptr;
	
#define SHOW_DUPLICATES		false
	
	// classify the macro according to its first word
	switch ( CMacroKind::MacroNameBeginningToID( gMacroKinds, word ) )
		{
		case CMacroKind::mExpression:
			{
			// expression macros begin with a double-quote
			RemoveQuotes( &s, word );
			
			// is it a duplicate?
			if ( CExpressionMacro::FindMacro( s.Get(), gRootExpressionMacro ) )
				{
#if SHOW_DUPLICATES
				ParseMsg( SHOW_DUPLICATES,
					/* "Duplicate expression macro %s." */
					_(TXTCL_MACROS_ERROR_DUPLICATEEXPRESSION), word );
#endif // SHOW_DUPLICATES
				return noErr;
				}
			
			ParseMsg( false,
				/* "Expression macro %s loaded." */
				_(TXTCL_MACROS_ERROR_EXPRESSIONLOADED), word );
			
			// create it
			macro = NEW_TAG("CExpressionMacro") CExpressionMacro( s.Get(), &gRootExpressionMacro );
			if ( not macro )
				return memFullErr;
			
			// load the next word
			line = NewWord( line, word );
			}
			break;
		
		case CMacroKind::mReplacement:
			{
			// replacement macros begin with an apostrophe
			RemoveQuotes( &s, word );
			
			// duplicate?
			CReplacementMacro * rmacro = CReplacementMacro::FindMacro( s.Get(),
				gRootReplacementMacro );
			if ( rmacro )
				{
				ParseMsg( SHOW_DUPLICATES,
					/* "Duplicate replacement macro %s." */
					_(TXTCL_MACROS_ERROR_DUPLICATEREPLACEMENT), word );
				return noErr;
				}
			ParseMsg( false,
				/* "Replacement macro %s loaded." */
				_(TXTCL_MACROS_ERROR_REPLACEMENTLOADED), word );
			
			// create it
			macro = NEW_TAG("CReplacementMacro") CReplacementMacro( s.Get(),
				&gRootReplacementMacro );
			if ( not macro )
				return memFullErr;
			
			// load the next word
			line = NewWord( line, word );
			}
			break;
		
		case CMacroKind::mVariable:
			{
			// variable macros begin with the word "set"
			
			// read the next word, the variable to be set
			line = NewWord( line, word );
			if ( not *word )
				return noErr;
			
			// disallow read-only variables, such as "set @name foo" etc.
			if ( CMacroKind::MacroNameToID( gGetMacroVariables, word ) )
				break;
			
			s.Set( word );
			
			// get the value to set it to
			line = NewWord( line, word );
			if ( not *word )
				return noErr;
			
			// perform the set
			CVariableMacro::SetVariable( s.Get(), word, &gRootVariableMacro );

			*word = '\0';
			}
			break;
		
		case CMacroKind::mIncludeFile:
			{
			// include-macros start with the word "include"
			// load name of file to load
			line = NewWord( line, word );
			if ( not *word )
				return noErr;
			
			// strip quotes, see if this file is already included
			RemoveQuotes( &s, word );
			CIncludeFileMacro * lmacro = CIncludeFileMacro::FindMacro( s.Get(),
												gRootIncludeFileMacro );
			if ( lmacro )
				{
				ParseMsg( SHOW_DUPLICATES,
					/* "Duplicate include of file %s." */
					_(TXTCL_MACROS_ERROR_DUPLICATEINCLUDE), word );
				return noErr;
				}
			ParseMsg( false,
				/* "File \"%s\" included." */
				_(TXTCL_MACROS_ERROR_FILEINCLUDED), word );
			
			// push this file onto the include stack
			lmacro = NEW_TAG("CIncludeFileMacro") CIncludeFileMacro( s.Get(),
														&gRootIncludeFileMacro );
			if ( not lmacro )
				return memFullErr;
			
			// Load this using a new parser
			CMacroParser parser;
			err = parser.ParseFile( s.Get() );
			
			*word = '\0';
			}
			break;
		
//		case CMacroKind::mKey:
		default:
			{
			// anything else has to be a key macro or a function macro, by definition;
			// but we can't tell which yet, until we see if it names a known key-combo
			s.Set( word );
			
			int key;
			uint modifiers;
			if ( GetKeyByName( s.Get(), &key, &modifiers ) )
				{
				// OK, it's a key macro -- but what sort? a click, wheel, or keyboard?
				if ( key >= kKeyClick && key < kKeyClickMax )
					{
					// Must be a click. check for dupes
					CKeyMacro * rmacro = CKeyMacro::FindMacro( key, modifiers, gRootClickMacro );
					if ( rmacro )
						{
						ParseMsg( SHOW_DUPLICATES,
							/* "Duplicate click macro '%s'." */
							_(TXTCL_MACROS_ERROR_DUPLICATECLICK), word );
						return noErr;
						}
					ParseMsg( false,
						/* "Click macro '%s' loaded." */
						_(TXTCL_MACROS_ERROR_CLICKLOADED), word );
					
					// create it
					macro = NEW_TAG("CKeyMacro") CKeyMacro( key, modifiers, &gRootClickMacro );
					}
				else
					{
					// must be a key macro. check for dupes.
					// (the wheelies are just a form of key macro.)
					CKeyMacro * rmacro = CKeyMacro::FindMacro( key, modifiers, gRootKeyMacro );
					if ( rmacro )
						{
						ParseMsg( SHOW_DUPLICATES,
								/* "Duplicate %s macro '%s'." */
								_(TXTCL_MACROS_ERROR_DUPLICATEMACRO),
								key < kKeyWheelUp ? "key" : "wheel",
								word );
						return noErr;
						}
					ParseMsg( false, /* "%s macro '%s' loaded." */
							_(TXTCL_MACROS_ERROR_MACROLOADED),
							key < kKeyWheelUp ? "Key" : "Wheel",
							word );
					
					// create it
					macro = NEW_TAG("CKeyMacro_Key") CKeyMacro( key, modifiers, &gRootKeyMacro );
					}
				if ( not macro )
					return memFullErr;
				
				line = NewWord( line, word );
				}
			else
				{
				// "after you've eliminated the impossible..."
				// this must be a function macro
				// check for duplicates
				CFunctionMacro * emacro = CFunctionMacro::FindMacro( word, gRootFunctionMacro );
				if ( emacro )
					{
					ParseMsg( SHOW_DUPLICATES,
						/* "Duplicate function \"%s\"." */
						_(TXTCL_MACROS_ERROR_DUPLICATEFUNCTION), word );
					return noErr;
					}
				ParseMsg( false,
					/* "Function \"%s\" loaded." */
					_(TXTCL_MACROS_ERROR_FUNCTIONLOADED), word );
				
				// create it
				macro = NEW_TAG("CFunctionMacro") CFunctionMacro( word, &gRootFunctionMacro );
				if ( not macro )
					return memFullErr;
				line = NewWord( line, word );
				}
			}
			break;
#undef SHOW_DUPLICATES
		}
	
	return err;
}


/*
**	CMacroParser::NewCommand()
*/
DTSError
CMacroParser::NewCommand( char * word, const char *& line )
{
	CMacro ** tMacro = &lastMacro->mTriggers;
	
	// Check to see if it is an attribute we need to set
	if ( int attrID = CMacroKind::MacroNameToID( gMacroAttributeKinds, word ) )
		{
		lastMacro->mAttributes |= attrID;
		*word = '\0';
		return noErr;
		}
	
	// It must be a command: namely, one of...
	//	pause, move, set, setglobal, call, end, if, else,
	//	random, or, label, goto, message, ignore_case
	// otherwise, treat it as a plain text command.
	
	int m_id = CMacroKind::MacroNameToID( gMacroCmdKinds, word );
	switch ( m_id )
		{
		case CMacroKind::cEnd:
			{
			// "end random"
			// "end if"
			// also "end" by itself would be accepted here but would cause errors
			line = NewWord( line, word );
			switch ( CMacroKind::MacroNameToID( gMacroCmdKinds, word ) )
				{
				case CMacroKind::cRandom:	m_id = CMacroKind::cEndRandom;		break;
				case CMacroKind::cIf:		m_id = CMacroKind::cEndIf;			break;
				}
			}
			break;
		
		case CMacroKind::cElse:
			{
			// "else"
			// "else if"
			line = NewWord( line, word );
			if ( CMacroKind::cIf == CMacroKind::MacroNameToID( gMacroCmdKinds, word ) )
				{
				CMacro * macro = NEW_TAG("CMacro") CMacro( m_id, tMacro );
				if ( not macro )
					return memFullErr;
				m_id = CMacroKind::cElseIf;
				}
			}
			break;
		
		case CMacroKind::cLabel:
			{
			// "label <name>"
			line = NewWord( line, word );
			CLabelCmdMacro * macro = NEW_TAG("CLabelCmdMacro") CLabelCmdMacro( word, tMacro );
			if ( not macro )
				return memFullErr;
			*word = '\0';
			return noErr;
			}
			break;
		
		case CMacroKind::cRandom:
			{
			// "random"
			// "random no-repeat"
			CRandomCmdMacro * macro = NEW_TAG("CRandomCmdMacro") CRandomCmdMacro( tMacro );
			if ( not macro )
				return memFullErr;
			line = NewWord( line, word );
			return noErr;
			}
			break;
		
		// If we didn't find one, load a cmd_Text and leave this word to be the parameter
		case 0:
			m_id = CMacroKind::cText;
			break;
		}
	
	CMacro * macro = NEW_TAG("CMacro") CMacro( m_id, tMacro );
	if ( not macro )
		return memFullErr;
	
	if ( CMacroKind::cText != m_id )
		line = NewWord( line, word );
	
	return noErr;
}


/*
**	CMacroParser::NewParameter()
**	slurp up the pending word as the parameter
*/
DTSError
CMacroParser::NewParameter( char * word, const char * &line )
{
	CMacro ** tMacro = &lastMacro->mTriggers;
	CParameterCmdMacro * macro = NEW_TAG("CParameterCmdMacro") CParameterCmdMacro( word, tMacro );
	if ( not macro )
		return memFullErr;
	
	line = NewWord( line, word );
	return noErr;
}


/*
**	CMacroParser::ParseMsg()
**	simplifying wrapper for ShowMacroInfoText(), to display parsing messages
**	Automatically supplies current line number & filename.
*/
void
CMacroParser::ParseMsg( bool mustShow, const char * fmt, ... )
{
	va_list params;
	va_start( params, fmt );
	CMacro::VShowMacroInfoText( mustShow, nullptr, currentLine, fileName.Get(), fmt, params );
	va_end( params );
}

#pragma mark -


/*
**	StopMacrosIfVar()
**	stop conditionally if a given variable is set
*/
static void
StopMacrosIfVar( const char * var )
{
	CExecutingMacro * on = gExecutingMacro;
	while ( on )
		{
		if ( CVariableMacro::TestBool( var, on->exVars ) )
			{
			CExecutingMacro * save = on;
			on = on->linkNext;
			delete save;
			}
		else
			on = on->linkNext;
		}
}


/*
**	StopMacrosOnKey()
**	a key was pressed. We'll stop but only if @env.key_interrupts is true
*/
void
StopMacrosOnKey()
{
	StopMacrosIfVar( kVar_Env_KeyIntr );
}


/*
**	DoMacroReplacement()
**	do replacements within macros
*/
void
DoMacroReplacement( int ch, uint modifiers )
{
	// If control is down, don't replace
	if ( modifiers & kKeyModControl )
		return;
	
	// On any non-alphanumeric, try to expand the last word
	if  ( not ( ( isprint(ch) && not isalnum(ch) )
			    || kEnterKey == ch || kReturnKey == ch ) )
		return;
	
	int startsel, endsel;
	gSendText.GetSelect( &startsel, &endsel );
	
	// Get the word right before the selection
	char buff [ kPlayerInputStrLen * 2 ];
	gSendText.GetText( buff, sizeof buff );
	size_t oldlen = strlen( buff );
	
	// Expand the LAST word if it was a return, the one before the selection if not
	size_t sel;
	if ( kEnterKey == ch || kReturnKey == ch )
		sel = oldlen;
	else
		sel = startsel;
	buff[ sel ] = '\0';
	
	// locate last word
	char * word = buff + sel - 1;
	while ( not strchr( kCLMacros_BreakString, * (const uchar *) word )  &&  word >= buff )
		--word;
	
	++word;
	
	// see if it's a replacement macro
	CReplacementMacro * macro = CReplacementMacro::FindMacro( word, gRootReplacementMacro );
	if ( not macro )
		return;
	
	// draw a discreet veil over our prestidigitations...
	gSendText.Hide();
	
	// Select the word we are expanding plus the space before it
	gSendText.SelectText( word - buff, startsel );
	
	if ( CExecutingMacro * emacro = StartMacroExecution( macro ) )
		{
		Continue1MacroExecution( emacro );
		
		// Only let it execute now or things get confusing
		if ( emacro )
			{
			CMacro::ShowMacroInfoText(
				/* "Replacement macro terminated. Pauses are not allowed." */
				_(TXTCL_MACROS_ERROR_REPLACEMENTTERMINATED) );
			delete emacro;
			}
		}
	
	// Restore the selection
	gSendText.GetText( buff, sizeof buff );
	int difflen = strlen( buff ) - oldlen;
	gSendText.SelectText( startsel + difflen, endsel + difflen );
	
	// raise the veil!
	gSendText.Show();
	
	// Annoying case of if we have erased everything and hit return - must erase!
	if ( 0 == gSendText.GetTextLength()
	&&	 ( kReturnKey == ch || kEnterKey == ch ) )
		{
		gSendText.SetText( " " );
		gSendText.SetText( "" );
		}
}


/*
**	DoMacroKey()
**	convert a keystroke to a macro
*/
bool
DoMacroKey( int * pch, uint modifiers )
{
	// control-escape stops all macros
	if ( kEscapeKey == * pch
	&&	 ( modifiers & kKeyModControl ) )
		{
		* pch = 0;
		FinishMacroExecution();
		return true;
		}
	
	int key = * pch;
	
	// (be careful not to lowerize pseudo-keys like kKeyClick & the wheelies)
	if ( key >= 'A' && key <= 'Z' )
		key = tolower( key );
	
	// See if this key triggered a new macro
	CKeyMacro * macro = CKeyMacro::FindMacro( key, modifiers & ~kKeyModRepeat, gRootKeyMacro );
	if ( not macro )
		return false;
	
	if ( not ( modifiers & kKeyModRepeat ) )
		{
		// Start the macro
		CExecutingMacro * emacro = StartMacroExecution( macro );
		if ( not emacro )
			return false;
		
		Continue1MacroExecution( emacro );
		}
	
	// We handled it... do we let the key pass?
	if ( macro->mAttributes & CMacroKind::aNoOverride )
		return false;

	* pch = 0;
	return true;
}


/*
**	DoMacroClick()
**	for clicks in the game window
*/
bool
DoMacroClick( const DTSPoint *	loc,
			  uint				modifiers,
			  int				button /* =1 */,
			  uint				chord  /* =1 */ )
{
	int key = kKeyClick + button - 1;		// button #s are 1-based
	
	// Check to see if we need to stop any executing macros
	StopMacrosIfVar( kVar_Env_ClickIntr );
	
	CKeyMacro * macro = CKeyMacro::FindMacro( key, modifiers, gRootClickMacro );
	if ( not macro )
		return false;
	
	// find the player who got clicked-upon
	const DescTable * desc = LocateMobileByPoint( loc, kDescPlayer );
	
	// if not on a player, ignore it -- unless we have the any_click attribute set.
	if ( not desc
	&&	 not ( macro->mAttributes & CMacroKind::aAnyClick ) )
		{
		return false;
		}
	
	CExecutingMacro * emacro = StartMacroExecution( macro );
	if ( not emacro )
		return false;
	
	// Add variables for the clicked-on character
	if ( desc )
		{
		// Load @click.name and @click.simple_name variables
		CVariableMacro::SetVariable( kVar_Click_name, desc->descName, &emacro->exVars, true );
		
		SafeString sclickname;
		sclickname.Set( desc->descName );
		RealToSimplePlayerName( sclickname.Get() );
		CVariableMacro::SetVariable( kVar_Click_sname, sclickname.Get(), &emacro->exVars, true );
		}
	else
		{
		CVariableMacro::SetVariable( kVar_Click_sname, "", &emacro->exVars, true );
		CVariableMacro::SetVariable( kVar_Click_name, "", &emacro->exVars, true );
		}
	
	// load the @click.button and @click.chord variables
	char temp[ 32 ];
	snprintf( temp, sizeof temp, "%d", button );
	CVariableMacro::SetVariable( kVar_Click_Button, temp, &emacro->exVars, true );
	snprintf( temp, sizeof temp, "%u", chord );
	CVariableMacro::SetVariable( kVar_Click_Chord, temp, &emacro->exVars, true );
	
	// let 'er rip!
	Continue1MacroExecution( emacro );
	
	// does this click override normal game-window semantics (select, befriend, etc.)?
	return not ( macro->mAttributes & CMacroKind::aNoOverride );
}


/*
**	DoMacroClick()
**	For clicks in the player window
*/
bool
DoMacroClick( const char * name, uint modifiers )
{
	int key = kKeyClick;
	
	CKeyMacro * macro = CKeyMacro::FindMacro( key, modifiers, gRootClickMacro );
	if ( not macro )
		return false;
	
	CExecutingMacro * emacro = StartMacroExecution( macro );
	if ( not emacro )
		return false;
	
	// Load @click.name and @click.simple_name variables
	CVariableMacro::SetVariable( kVar_Click_name, name, &emacro->exVars );
	
	SafeString sclickname;
	sclickname.Set( name );
	RealToSimplePlayerName( sclickname.Get() );
	CVariableMacro::SetVariable( kVar_Click_sname, sclickname.Get(), &emacro->exVars );
	
	// button & chord don't apply (not yet anyway)
	
	// fire off the macro
	Continue1MacroExecution( emacro );
	
	// decide whether the original window should see the click too
	return not ( macro->mAttributes & CMacroKind::aNoOverride );
}


/*
**	DoMacroText()
**
**	Invoke expression macros starting a line of text
**	Returns true if the macro was completely handled
*/
bool
DoMacroText( const char * sendbuff )
{
	// Look for an expression matching the first word of the line
	char word[ kPlayerInputStrLen ];
	CMacroParser::GetWord( sendbuff, word, sizeof word, "" );
	
	CExpressionMacro * macro = CExpressionMacro::FindMacro( word, gRootExpressionMacro );
	if ( not macro )
		return false;
	
	// Send the text command that was just typed to the command list - but don't execute
	char buff[ kPlayerInputStrLen ];
	gSendText.GetText( buff, sizeof buff );
	gCmdList->SendCmd( true, buff, false );
	
	CExecutingMacro * emacro = StartMacroExecution( macro );
	if ( not emacro )
		return false;
	
	Continue1MacroExecution( emacro );
	
	// Tell the sent text that we handled it
	return true;
}


//
//	Macro interface routines for starting and running macros
//


/*
**	StartMacroExecution()
**	start up a macro; create an execution context for it
*/
CExecutingMacro *
StartMacroExecution( CTriggerMacro * macro )
{
	CExecutingMacro * on = NEW_TAG("CExecutingMacro") CExecutingMacro( macro );
	if ( not on )
		{
		// out of memory
	 	Beep();
	 	}
	
	return on;
}


/*
**	Continue1MacroExecution()
**	single-step a macro via its execution context
*/
void
Continue1MacroExecution( CExecutingMacro *& on )
{
	if ( not on )
		return;
	
	if ( /* DTSError err = */ on->Continue() )
		{
		delete on;
		on = nullptr;
		}
}


/*
**	ContinueMacroExecution()
**	run a macro until done
*/
void
ContinueMacroExecution()
{
	CExecutingMacro * on = gExecutingMacro;
	
	while ( on )
		{
		CExecutingMacro * save = on;
		on = on->linkNext;
		Continue1MacroExecution( save );
		}
	gSendText.UpdateMacroStatus( gExecutingMacro != nullptr );
}


/*
**	FinishMacroExecution()
**	clean up after
*/
void
FinishMacroExecution()
{
	CExecutingMacro::DeleteLinkedList( gExecutingMacro );
	gSendText.UpdateMacroStatus( gExecutingMacro != nullptr );
}


/*
**	InterruptMacro()
**
**	cause a macro to stop
*/
void
InterruptMacro()
{
	CExecutingMacro::sExInterrupted = true;
}


/*
**	SetMacroTextWinBuffer()
**
**	remember a line of text from the window
*/
void
SetMacroTextWinBuffer( const char * text )
{
	CMacro::SetTextWinBuffer( text );
}

#pragma mark -


/*
**	CExecutingMacro::CExecutingMacro()
**	constructor for macro execution context
*/
CExecutingMacro::CExecutingMacro( CTriggerMacro * macro ) :
	exMark( nullptr ),
	exVars( nullptr ),
//	exBuffer(),
	exWaitUntil( 0 ),
	exKind( macro->mKind )
//	, exDebug( false )
//	, exUnfriendly( false )
{
	CExecutingMacro::InstallLast( gExecutingMacro );
	
	exDebug			= CVariableMacro::TestBool( kVar_Env_Debug, exVars );
	exUnfriendly	= CVariableMacro::TestBool( kVar_Env_Unfriendly, exVars );
	sExInterrupted	= false;
	sNumGotos		= 0;
	
	// Save the text
	char buff[ kPlayerInputStrLen ];
	gSendText.GetText( buff, sizeof buff );
	
	// If it is an expression, remove the shorthand
	char * tempbuff = buff;
	if ( CMacroKind::mExpression == exKind )
		{
		CExpressionMacro * emacro = static_cast<CExpressionMacro *>( macro );
		if ( strlen( buff ) > emacro->mExpression.Size() )
			tempbuff = buff + emacro->mExpression.Size();
		else
			*tempbuff = '\0';
		}
	// save text in @text
	CVariableMacro::SetVariable( kVar_Text, tempbuff, &exVars, true );
	
	// Save the selected text in @textsel
	int selstart, selend;
	gSendText.GetSelect( &selstart, &selend );
	if ( selstart < selend )
		{
		size_t l = (selend - selstart) > (int) sizeof buff ?
			sizeof buff : size_t(selend - selstart);
		strncpy( buff, buff + selstart, l );
		if ( sizeof buff - l > 1 )
			buff[l] = '\0';
		}
	CVariableMacro::SetVariable( kVar_TextSel, buff, &exVars, true );
	
	// Initialize the linked list of routines we are in
	exMark = NEW_TAG("CMarkMacro") CMarkMacro( reinterpret_cast<CMacro **>( &exMark ) );
	if ( exMark )
		exMark->mFirst = exMark->mTriggers = macro->mTriggers;
}


/*
**	CExecutingMacro::~CExecutingMacro()
**	destroy an execution context
*/
CExecutingMacro::~CExecutingMacro()
{
	// Insert anything left in exBuffer to the SendText
	if ( *exBuffer.Get() )
		{
		// Replacement macros must insert text. If they don't insert something, remove the word
		if  ( CMacroKind::mReplacement == exKind )
			gSendText.Insert( exBuffer.Get() );
		else
		if ( not strchr( exBuffer.Get(), '\r' ) )
			{
			if ( CMacroKind::mExpression == exKind )
				gSendText.SetText( exBuffer.Get() );
			else
				gSendText.Insert( exBuffer.Get() );
			}
		}
	
	// Extend the selection back one space and delete if the replacement sends no text
	else
	if ( CMacroKind::mReplacement == exKind )
		{
		int startsel, endsel;
		gSendText.GetSelect( &startsel, &endsel );
		gSendText.SelectText( startsel - 1, endsel );
		gSendText.Clear();
		}
	
	Remove( gExecutingMacro );
	
	CMarkMacro::DeleteLinkedList( reinterpret_cast<CMacro *&>( exMark ) );
	CMacro::DeleteLinkedList( exVars );
}


/*
**	CExecutingMacro::Continue()
**	perform one step/frame of the macro
*/
DTSError
CExecutingMacro::Continue()
{
	// macro interrupted by server?
	if ( sExInterrupted )
		{
		FinishMacroExecution();
		return noErr;
		}
	
	// still in a pause?
	if ( exWaitUntil )
		{
		if ( exWaitUntil > gAckFrame )
			return noErr;
		exWaitUntil = 0;
		}
	
	ulong time = GetFrameCounter();
	
	while ( not exWaitUntil )
		{
		// Let CL process events
		if ( not exUnfriendly )
			{
			if ( GetFrameCounter() - time > 5 )
				return noErr;
			}
		else
			{
			// Let it go for as long as it wants to, but still break on control escape
			if ( IsKeyDown( kControlModKey ) && IsKeyDown( kEscapeKey ) )
				return noErr;
			}
		
		// Send any text which is in the buffer with a \r after it.
		char * foundReturn = strchr( exBuffer.Get(), '\r' );
		if ( foundReturn )
			{
			// Replacement macros can't do this
			if ( CMacroKind::mReplacement == exKind )
				{
				CMacro::ShowMacroInfoText( /* "Execution Error\r"
							"Replacement macros may not have a return '\\r' in them." */
							_(TXTCL_MACROS_ERROR_REPLACEMENTWITHRETURN) );
				
				// Kill the return and after
				*foundReturn = '\0';
				return 1;
				}
		
			// Send up to the next \r to the TextCmdList unless we can't
			char toSend[ kPlayerInputStrLen ];
#if 0
	// this will overwrite 'toSend', and completely trash the stack,
	// if the exBuffer text is too long.
			strncpy( toSend, exBuffer.Get(), foundReturn - exBuffer.Get() + 1 );
			toSend[ foundReturn - exBuffer.Get() + 1 ] = '\0';
#else
			size_t toSendLen = foundReturn - exBuffer.Get() + 1;
			if ( toSendLen >= sizeof toSend )
				{
				// "Execution Error\rMacro text too long; %u characters truncated.\r"
				// "Offending text was: \"%s\""
				CMacro::ShowMacroInfoText( _(TXTCL_MACROS_ERROR_TEXT_TRUNCATED),
							uint( toSendLen - sizeof toSend - 1 ),
							exBuffer.Get() );
				
				toSendLen = sizeof toSend;
				}
			StringCopySafe( toSend, exBuffer.Get(), toSendLen );
			
			// if we truncated that \r, reinsert it.
			// Of course, once we've done any truncation at all, then we're starting to
			// skate on very thin ice, and the only truly -safe- thing to do would be
			// to halt now, with a nagging message asking the user to repair his macros.
			if ( toSendLen >= sizeof toSend )
				toSend[ sizeof toSend - 2 ] = '\r';	 // stick it before the '\0', at ts[-1]
#endif  // 0
			
			// If @echo is true set the text (without the ending \r) and select all of it
			bool echo = CVariableMacro::TestBool( kVar_Env_Echo, exVars );
			
			if ( gCmdList->SendCmd( echo, toSend ) )
				{
				if ( echo )
					{
					toSend[ toSendLen - 1 ] = '\0';
					gSendText.SetText( toSend );
					gSendText.SelectText( 0, 0x7FFF );
					}
				exBuffer.Set( foundReturn + 1 );
				}
			
			// return now; no reason to send more than one command per frame
			return noErr;
			}
		
		// See if the lowest level function is done
		DTSError err = noErr;
		CMarkMacro ** on = &exMark;
		while ( (*on)->linkNext )
			on = reinterpret_cast<CMarkMacro **>( &(*on)->linkNext );
		
		// Check to see if we are done or an error has occurred
		while ( err || not (*on)->mTriggers )
			{
			err = noErr;
			
			// Delete this function
			CMarkMacro::DeleteLinkedList( (CMacro *&) *on );
			*on = nullptr;
			
			// Check the next highest to see if it is done and so on.
			if ( not exMark )
				return 1;
			
			on = &exMark;
			while ( (*on)->linkNext )
				on = reinterpret_cast<CMarkMacro **>( &(*on)->linkNext );
			}
		
		// Execute a command
		err = ExecuteCommand( &(*on)->mTriggers, (*on)->mFirst );
		}
	
	return noErr;
}


/*
**	CExecutingMacro::GetParameters()
**	Loads all of the parameters and increments exMacro to the next command
*/
DTSError
CExecutingMacro::GetParameters( CMacro ** exMacro, MacroParamRec * ioParams, int& ioParamsCount )
{
	int ii = 0;
	
	// Copy the parameters
	while ( *exMacro
	&&		CMacroKind::cParameter == (*exMacro)->mKind
	&&		ii < ioParamsCount )
		{
		const CParameterCmdMacro * pmacro = static_cast<CParameterCmdMacro *>( *exMacro );
		CopyExpression( &(*ioParams)[ii], pmacro->mParam.Get(), exVars );
		
		++ii;
		*exMacro = (*exMacro)->linkNext;
		}
	
	ioParamsCount = ii;
	return noErr;
}


/*
**	CExecutingMacro::Pause()
**	pause execution for n frames
*/
void
CExecutingMacro::Pause( int frames )
{
	if ( frames > 0 )
		exWaitUntil = gAckFrame + frames;
}


/*
**	CExecutingMacro::ExecuteCommand()
**	just what it sounds like
*/
DTSError
CExecutingMacro::ExecuteCommand( CMacro ** exMacro, CMacro * exStart )
{
	if ( not *exMacro )
		return noErr;
	
	MacroParamRec params;
	SafeString cmdName;
	
	// Save the ID because exMacro will change with getting parameters
	int m_id = (*exMacro)->mKind;
	
	CMacro * cmdMacro = *exMacro;
	* exMacro = (*exMacro)->linkNext;
	
	int paramsCount = kCLMacros_MaxCmdParam;
	DTSError err = GetParameters( exMacro, &params, paramsCount );
	if ( err )
		return err;
	
	CMacroKind::MacroIDToName( gMacroCmdKinds, m_id, &cmdName );
	
	switch ( m_id )
		{
		case CMacroKind::cPause:
			{
			if ( 1 != paramsCount )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\r%s <number of frames>", cmdName.Get() );
				err = 1;
				break;
				}
			
			int delay;
			if ( not StringToNum( params[0].Get(), delay ) )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\r"
									"%s <number of frames>\r'%s' is not a number.",
									cmdName.Get(), params[0].Get() );
				err = 1;
				break;
				}
			
			Pause( delay );
			}
			break;
		
		case CMacroKind::cMove:
			{
			// move stop
			// move <speed> <direction>
			static const char * const names[] =
				{ "stop", "E", "NE", "N", "NW", "W", "SW", "S", "SE" };
			static const char * const names2[] =
				{ "stop", "east", "northeast", "north", "northwest",
				  "west", "southwest", "south", "southeast" };
			if ( paramsCount == 1 )
				{
				// must be "stop"
				if ( 0 == strcasecmp( params[0].Get(), names[0] ) )
					MovePlayerToward( move_Stop, false, false );
				else
					{
					CMacro::ShowMacroInfoText( "Syntax Error\rShould be '%s'", names[0] );
					err = 1;
					break;
					}
				}
			else
			if ( 2 != paramsCount )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\r"
							"%s walk/run <direction>", cmdName.Get() );
				err = 1;
				break;
				}
			else
				{
				// get desired speed
				bool fast = false;
				int quadrant = 0;
				if ( not strcasecmp( params[0].Get(), "run" ) )
					fast = true;
				else
				if ( not strcasecmp( params[0].Get(), "walk" ) )
					fast = false;
				else
					{
					CMacro::ShowMacroInfoText(
							"Syntax Error\r%s walk/run <direction>\r'%s' is not 'walk' or 'run'",
							cmdName.Get(), params[0].Get() );
					err = 1;
					break;
					}
				
				// get desired direction
				bool gotIt = false;
				const int kNumQuadrants = sizeof names / sizeof names[0];
				for ( int ii = 0;  ii < kNumQuadrants && not gotIt; ++ii )
					if ( 0 == strcasecmp( params[1].Get(), names[ii] )
					||	 0 == strcasecmp( params[1].Get(), names2[ii] ) )
						{
						quadrant = ii;
						gotIt = true;
						break;
						}
				if ( gotIt )
					MovePlayerToward( quadrant, fast, false );
				else
					{
					CMacro::ShowMacroInfoText( "Syntax Error\r%s %s <direction>\r"
									"'%s' is not a valid direction",
									cmdName.Get(), params[0].Get(), params[1].Get() );
					err = 1;
					break;
					}
				}
			}
			break;
		
		case CMacroKind::cSetGlobalVariable:
		case CMacroKind::cSetVariable:
			{
			// set var value
			// set var op value
			
			// local or global?
			CMacro ** vars;
			if ( CMacroKind::cSetVariable == m_id )
				vars = &exVars;
			else
				vars = &gRootVariableMacro;
			
			// We have to reconstitute the first variable in case it was a variable name which
			// was resolved to a value during normal parameter handling
			CParameterCmdMacro * pmacro = static_cast<CParameterCmdMacro *>( cmdMacro->linkNext );
			if ( pmacro )
				params[0].Set( pmacro->mParam.Get() );
			
			if ( 2 == paramsCount )
				{
				// easy case: set var value
				CVariableMacro::SetVariable( params[0].Get(), params[1].Get(), vars );
				}
			else
			if ( 3 == paramsCount )
				{
				// hard case: math/string operations
				static const char * const operations[] = { "+", "-", "*", "/", "%" };
				enum macroOps {
					kOpAdd, kOpSubtract, kOpMultiply, kOpDivide, kOpModulo, kNum_Set_Ops
				};
				bool gotIt = false;
				int opNum;
				for ( int ii = 0;  ii < kNum_Set_Ops && not gotIt;  ++ii )
					if ( 0 == strcasecmp( params[1].Get(), operations[ii] ) )
						{
						opNum = ii;
						gotIt = true;
						break;
						}
				
				if ( not gotIt )
					{
					CMacro::ShowMacroInfoText( "Syntax Error\r"
							"%s <value> <operation> <value> - '%s' is not a recognized operation.",
							cmdName.Get(), params[1].Get() );
					err = 1;
					break;
					}
				
				// get LHS of expression
				SafeString value1;
				if ( not CVariableMacro::GetVariable( params[0].Get(), &value1, exVars ) )
					{
					CMacro::ShowMacroInfoText( "Syntax Error\r"
								"%s <value> <operation> <value> - the variable %s is not defined.",
								cmdName.Get(), params[0].Get() );
					err = 1;
					break;
					}
				
				// check operation is permissible for LHS
				int num1(0), num2(0);
				bool op1String = false;
				bool op2String = false;
				if ( not StringToNum( value1.Get(), num1 ) )
					{
					if ( kOpAdd == opNum )
						{
						// string addition is allowed
						op1String = true;
						}
					else
						{
						// all other operations require numeric arguments
						CMacro::ShowMacroInfoText( "Syntax Error\r"
									"%s <value> <operation> <value>\r%s is not a number.",
									cmdName.Get(), value1.Get() );
						err = 1;
						break;
						}
					}
				
				// also check operation is permissible for RHS
				if ( not StringToNum( params[2].Get(), num2 ) )
					{
					if ( kOpAdd == opNum
					&&	 op1String )
						{
						// string concatenation allowed if arg1 was also a string
						op2String = true;
						}
					else
						{
						CMacro::ShowMacroInfoText( "Syntax Error\r"
									"%s <value> <operation> <value>\r%s is not a number.",
									cmdName.Get(), params[2].Get() );
						err = 1;
						break;
						}
					}
				
				// perform it
				switch ( opNum )
					{
					case kOpAdd:
						// slightly tricky one: handle string addition
						if ( op1String && op2String )
							{
							// string + string
							value1.Append( params[2].Get() );
							}
						else
						if ( op1String && not op2String )
							{
							// string + number
							value1.Append( num2 );
							}
						else
						if ( not op1String && op2String )
							{
							// number + string
							CMacro::ShowMacroInfoText( "Syntax Error\r"
										"%s <number> + <string> not allowed.", cmdName.Get() );
							err = 1;
							break;
							}
						else
							{
							// number + number
							num1 = num1 + num2;
							}
						break;
					
					// all the rest are pieces of cake
					case kOpSubtract:	num1 = num1 - num2;	break;
					case kOpMultiply:	num1 = num1 * num2;	break;
					case kOpDivide:		num1 = num1 / num2;	break;
					case kOpModulo:		num1 = num1 % num2;	break;
					}
				
				// don't convert string concatenations
				if ( not op1String )
					value1.Set( num1 );
				
				// save new value back into LHS
				CVariableMacro::SetVariable( params[0].Get(), value1.Get(), vars );
				}
			else
				{
				// wrong number of params
				CMacro::ShowMacroInfoText( "Syntax Error\r"
							"%s <variable> <value> OR %s <variable> <operation> <value>",
							cmdName.Get(), cmdName.Get() );
				err = 1;
				break;
				}
			
			// Check our debug variable (this is better than checking it for every single command)
			exDebug = CVariableMacro::TestBool( kVar_Env_Debug, exVars );
			exUnfriendly = CVariableMacro::TestBool( kVar_Env_Unfriendly, exVars );
			}
			break;
		
		case CMacroKind::cCallFunction:
			{
			// "call <function-macro-name>"
			if ( 1 != paramsCount )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\r%s <function>", cmdName.Get() );
				err = 1;
				break;
				}
			
			if ( CFunctionMacro * fmacro = CFunctionMacro::FindMacro( params[0].Get(),
					gRootFunctionMacro ) )
				{
				// Add a new node to the executing macro's mark
				CMarkMacro * mmacro = NEW_TAG("CMarkMacro") CMarkMacro(
											reinterpret_cast<CMacro **>( &exMark ));
				if ( not mmacro )
					{
					/* "Out of memory." */
					CMacro::ShowMacroInfoText( _(TXTCL_MACROS_ERROR_OUTOFMEMORY) );
					err = memFullErr;
					break;
					}
				mmacro->mFirst = mmacro->mTriggers = fmacro->mTriggers;
				}
			else
				{
				// never heard of such a function
				CMacro::ShowMacroInfoText(
							/* "Execution Error\r'%s' not a defined function." */
							_(TXTCL_MACROS_ERROR_NOTDEFINEDFUNCTION),
							params[0].Get() );
				err = 1;
				}
			}
			break;
		
		
		//
		// If family of commands
		//
		case CMacroKind::cElseIf:
		case CMacroKind::cIf:
			{
			// "if <value> <compare_op> <value>"
			// "elseif <value> <compare_op> <value>"
			
			bool passed = false;
			if ( 3 != paramsCount )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\r%s <value> <comparison> <value>",
							cmdName.Get() );
				err = 1;
				}
			else
				{
				// determine which comparison operation
				static const char * const comparisons[] = { ">", "<", ">=", "<=", "==", "!=" };
				enum CompOps { kOpGreater, kOpLess,
							   kOpGreaterEq, kOpLessEq,
							   kOpEqual, kOpNotEqual,
							   kNum_Comp_Ops };
				bool gotIt = false;
				int compNum;
				for ( int ii = 0; ii < kNum_Comp_Ops && not gotIt; ++ii )
					if ( 0 == strcasecmp( params[1].Get(), comparisons[ii] ))
						{
						compNum = ii;
						gotIt = true;
						break;
						}
				
				if ( not gotIt )
					{
					CMacro::ShowMacroInfoText( "Syntax Error\r"
							"%s <value> <comparison> <value> - "
							"'%s' is not a recognized comparison.",
							cmdName.Get(), params[1].Get() );
					err = 1;
					break;
					}
				
				// numeric or string comparison?
				int num1(0), num2(0);
				bool isnum1 = StringToNum( params[0].Get(), num1 );
				bool isnum2 = StringToNum( params[2].Get(), num2 );
				
				bool bothExp = not isnum1 || not isnum2;
				
				// perform it
				switch ( compNum )
					{
					case kOpGreater:
						if ( bothExp )
							passed = strstr( params[2].Get(), params[0].Get() )
									 && ( 0 != strcasecmp( params[0].Get(), params[2].Get()) );
						else
							passed = ( num1 > num2 );
						break;
					
					case kOpLess:
						if ( bothExp )
							passed = strstr( params[0].Get(), params[2].Get() )
									 && ( 0 != strcasecmp( params[0].Get(), params[2].Get()) );
						else
							passed = ( num1 < num2 );
						break;
					
					case kOpGreaterEq:
						if ( bothExp )
							passed = ( nullptr != strstr( params[0].Get(), params[2].Get()) );
						else
							passed = ( num1 >= num2 );
						break;
					
					case kOpLessEq:
						if ( bothExp )
							passed = ( nullptr != strstr( params[2].Get(), params[0].Get() ) );
						else
							passed = ( num1 <= num2 );
						break;
					
					case kOpEqual:
						if ( bothExp )
#ifdef MULTILINGUAL
							passed = ( 0 == CompareStrVariablewithShortTag( params[0].Get(),
												params[2].Get() ) );
#else
							passed = ( 0 == strcasecmp( params[0].Get(), params[2].Get() ) );
#endif // MULTILINGUAL
						else
							passed = ( num1 == num2 );
						break;
					
					case kOpNotEqual:
						if ( bothExp )
#ifdef MULTILINGUAL
							passed = ( 0 != CompareStrVariablewithShortTag( params[0].Get(),
												params[2].Get() ) );
#else
						passed = ( 0 != strcasecmp( params[0].Get(), params[2].Get() ) );
#endif // MULTILINGUAL
					else
						passed = ( num1 != num2 );
					break;
					}
				}
			
			if ( not passed )
				{
				// Move up to the next else or the end
				const int cmdSet[2] = { CMacroKind::cEndIf, CMacroKind::cElse };
				FindCmdOnSameLevel( exMacro, 2, cmdSet );
				
				if ( not *exMacro )
					{
					CMacro::ShowMacroInfoText( "Syntax Error\rNo closing \"end if\" found." );
					err = 1;
					break;
					}
				
				// Move forward from an else so it will actually execute
				if ( CMacroKind::cElse == (*exMacro)->mKind )
					*exMacro = (*exMacro)->linkNext;
				}
			}
			break;
		
		case CMacroKind::cElse:
			{
			const int cmdSet[1] = { CMacroKind::cEndIf };
			if ( not FindCmdOnSameLevel( exMacro, 1, cmdSet ) )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\rNo closing \"end if\" found." );
				err = 1;
				break;
				}
			cmdName.Set( "end if" );
			*exMacro = (*exMacro)->linkNext;
			}
			break;
		
		case CMacroKind::cEndIf:
			cmdName.Set( "end if" );
			break;
		
		//
		// Random family of commands
		//
		case CMacroKind::cRandom:
			{
			// "random" [ "no-repeat" ]
			int lastChosen, chosen;
			CRandomCmdMacro	* rmacro = nullptr;
			
			if ( 1 == paramsCount )
				{
				if ( 0 == strcasecmp( params[0].Get(), kMod_No_Repeat ) )
					{
					rmacro = static_cast<CRandomCmdMacro *>( cmdMacro );
					lastChosen = rmacro->mLastChosen;
					}
				else
					{
					CMacro::ShowMacroInfoText( "Syntax Error\r"
								"%s <option>, '%s' is not a recognized option.",
								cmdName.Get(), params[0].Get() );
					err = 1;
					break;
					}
				}
			else
			if ( 0 == paramsCount )
				lastChosen = -1;
			else
				{
				CMacro::ShowMacroInfoText( "Syntax Error\r%s <option>", cmdName.Get() );
				err = 1;
				break;
				}
			
			// Count the number of Or's before the next End_Random
			int numOr = 1;
			CMacro * temp = *exMacro;
			
			const int cmdSet[2] = { CMacroKind::cOr, CMacroKind::cEndRandom };
			while ( FindCmdOnSameLevel( &temp, 2, cmdSet )
			&&		CMacroKind::cEndRandom != temp->mKind )
				{
				++numOr;
				temp = temp->linkNext;
				}
			
			if ( not temp )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\rNo ending \"end random\" found." );
				err = 1;
				break;
				}
			
			// Get a random number that wasn't the last chosen
			// (but only if more than one alternative!)
			if ( numOr > 1 )
				{
				do
					{
					chosen = GetRandom( numOr );
					} while ( chosen == lastChosen );
				}
			else
				chosen = 0;
			
			// Save which one was chosen
			if ( rmacro )
				rmacro->mLastChosen = chosen;
			
			// Move exMacro to the chosen one
			numOr = 0;
			
			const int cmdSet2[1] = { CMacroKind::cOr };
			while (	numOr < chosen
			&&		FindCmdOnSameLevel( exMacro, 1, cmdSet2 ) )
				{
				++numOr;
				if ( *exMacro )
					*exMacro = (*exMacro)->linkNext;
				}
			}
			break;
		
		case CMacroKind::cOr:
			{
			// "or"
			const int cmdSet[1] = { CMacroKind::cEndRandom };
			if ( not FindCmdOnSameLevel( exMacro, 1, cmdSet ) )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\rNo closing \"end random\" found." );
				err = 1;
				break;
				}
			cmdName.Set( kEnd_Random );
			*exMacro = (*exMacro)->linkNext;
			}
			break;
		
		case CMacroKind::cEndRandom:
			// "end random"
			cmdName.Set( kEnd_Random );
			break;
		
		case CMacroKind::cGoto:
			{
			// "goto <label>"
			if ( 1 != paramsCount )
				{
				CMacro::ShowMacroInfoText( "Syntax Error\r %s <label>", cmdName.Get()
					/*, cmdName.Get() */ );
				err = 1;
				break;
				}
			
			// Resolve variables
			CLabelCmdMacro * lmacro = CLabelCmdMacro::FindMacro( params[0].Get(), exStart );
			if ( lmacro )
				{
				*exMacro = lmacro;
				++sNumGotos;
				}
			else
				{
				CMacro::ShowMacroInfoText( /* "Execution Error\r"
							"label %s not found in the current macro." */
							_(TXTCL_MACROS_ERROR_LABELNOTFOUND),
							/* cmdName.Get(), */ params[0].Get() );
				err = 1;
				}
			}
			break;
		
		case CMacroKind::cText:
			// Copy each parameter into the end of exBuffer
			for ( int ii = 0;  ii < paramsCount;  ++ii )
				exBuffer.Append( params[ii].Get() );
			break;
		}
	
	// Show a message
	if ( CMacroKind::cMessage == m_id )
		{
		cmdName.Clear();
		for ( int ii = 0;  ii < paramsCount;  ++ii )
			{
			if ( ii )
				cmdName.Append( " " );
			cmdName.Append( params[ii].Get() );
			}
		ShowInfoText( cmdName.Get() );
		}
	
	// Show what we did if debugging or there was an error
	else
	if ( ( *cmdName.Get() && exDebug ) || err )
		{
		for ( int ii = 0;  ii < paramsCount;  ++ii )
			cmdName.Format( " %s", params[ii].Get() );
		CMacro::ShowMacroInfoText( "%s", cmdName.Get() );
		}
	
	return err;
}


//
// Find the next command of a given set of types, setting the passed macro to it.
// This implements instruction skipping in if-else-endif and random...endrandom constructs
//
bool
CExecutingMacro::FindCmdOnSameLevel( CMacro ** start, int numCmds, const MacroCmdSet cmdSet )
{
	int cmdLevel = 0;		// keep track of nesting levels
	
	// scan forward thru linked list
	for (  ;  *start;  *start = (*start)->linkNext )
		{
		// have we found a good one?
		if ( 0 == cmdLevel )
			{
			for ( int ii = 0;  ii < numCmds;  ++ii )
				{
				if ( (*start)->mKind == cmdSet[ii] )
					return true;
				}
			}
		
		// Handle commands which change the level
		switch ( (*start)->mKind )
			{
			case CMacroKind::cIf:
			case CMacroKind::cRandom:
				++cmdLevel;
				break;
			
			case CMacroKind::cEndIf:
			case CMacroKind::cEndRandom:
				--cmdLevel;
				break;
			}
		}
	
	return false;
}


#ifdef MULTILINGUAL
/*
**	CompareStrVariablewithShortTag
**	compare two strings where the second string can be a
**	item short tag description (e.g. %lfcy)
*/
bool
CompareStrVariablewithShortTag( const char * value1, const char * value2 )
{
	/* To allow multilingual itemnames in macros we are
	   having special item shortnames hidden inside the item-
	   public names. These look like '%lfcy' (stands for lifecrystal)
	   and should get expanded into the real translated item name here.
	   So @my.left_item >= "Lifecrystal" can be written as
	   @my.left_item >= "%lfcy" which works for all languages.
	*/
	char buff[ 256 ];
//	ShowMessage( "CompareVariable: %s = %s", value1, value2 );
	
	if ( strlen( value2 ) == 5 && '%' == value2[0] )
		{
		GetInvItemByShortTag( value2, buff, sizeof buff );
//		ShowMessage( "CompareVariable2: Got buff: %s", buff );
		return strcasecmp( value1, buff );
		}
	
	return strcasecmp( value1, value2 );
}
#endif // MULTILINGUAL

