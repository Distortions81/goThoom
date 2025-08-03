/*
**	SendText_cl.cp		ClanLord
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
#include "Macros_cl.h"
#include "Movie_cl.h"
#include "SendText_cl.h"
#include "TextCmdList_cl.h"


/*
**	Global Variables
*/
SendTextField	gSendText;


/*
**	Internal Routines
*/
namespace
{
	bool		DoSpecialKey( int ch );
	void		UseCachedString( int delta );
	bool		DoNormalKey( int ch, uint modifiers );
	void		DoEnterKey();
#if ! CLIENT_COMMANDS
	void		LogTextCmd( const char * cmd );
#endif
	DTSError	InstallTextHandler();
}


/*
**	SendTextField::DoDraw()
**	Outline the text field if a macro is in progress
*/
void
SendTextField::DoDraw()
{	
	DTSTextField::DoDraw();
	
	DTSRect rect;
	GetBounds( &rect );
	
	rect.Inset( -3, -3 );
	++rect.rectBottom;
	SetPenSize( 3 );

	if ( lastDrawnMacroExecuting )
		{
		SetForeColor( &DTSColor::grey50 );
		Frame( &rect );
		SetForeColor( &DTSColor::black );
		}
	else
		{
		SetPenMode( kPenModeErase );
		Frame( &rect );
		}
	
	SetPenMode( kPenModeCopy );
}


/*
**	SendTextField::UpdateMacroStatus()
*/
void
SendTextField::UpdateMacroStatus( bool macroExecuting )
{
	if ( macroExecuting != lastDrawnMacroExecuting )
		{
		lastDrawnMacroExecuting = macroExecuting;
		Draw();
		}
}


/*
**	InitSendText()
**
**	initialize the send text field
*/
void
InitSendText( DTSWindow * win, const DTSRect * box )
{
	// inset the box by the margin
	DTSRect bounds( *box );
	bounds.Inset( kGameInputMargin, kGameInputMargin );
	
	// initialize the text field and show it
	gSendText.Hide();
	Str255 fontName;
	SInt16 fSize;
	Style fStyle;
	__Verify_noErr( GetThemeFont( kThemeApplicationFont, smRoman, fontName, &fSize, &fStyle ) );
	
	char cFontName[ 256 ];
	CopyPascalStringToC( fontName, cFontName );
	
	gSendText.Init( win, &bounds, cFontName, fSize );
	gSendText.Show();
}


/*
**	SendTextField::Init()
**	initialize the input text field
*/
void
SendTextField::Init( DTSView * parent, const DTSRect * box, const char * font, int sz )
{
	// call thru to superclass
	DTSTextField::Init( parent, box, font, sz );
	
	// these must only be done once
	static bool bFieldInited;
	if ( not bFieldInited )
		{
		bFieldInited = true;
		
		// add the appleEventHandler
		__Verify_noErr( InstallTextHandler() );
		
		// install the service handler
		WindowRef win = DTS2Mac( static_cast<DTSWindow *>( parent ) );
		OSStatus result = InitTarget( GetWindowEventTarget( win ) );
		if ( noErr == result )
			{
			const EventTypeSpec evts[] =
				{
					{ kEventClassService, kEventServiceGetTypes },
					{ kEventClassService, kEventServiceCopy },
					{ kEventClassService, kEventServicePaste }
				};
			result = AddEventTypes( GetEventTypeCount( evts ), evts );
			}
		}
	
	lastDrawnMacroExecuting = false;
}


/*
**	GetSendString()
**
**	copy the send text field to the destination buffer
**	cache the send string
**	clear the send string
*/
void
GetSendString( char * dst, size_t /* bufSize */ )
{
	if ( gCmdList->GetCmdToSend( gCommandNumber, dst ) )
		{
//		ShowMessage( "Got a cmd %u: %s", (uint) gCommandNumber, dst );
		}
}


/*
**	ClearSendString()
**
**	clear the send string
*/
void
ClearSendString( uint ackCmdNum )
{
	if ( gCmdList->CmdDone( ackCmdNum ) )
		{
		//	ShowMessage("Ack a cmd");
		}
}


/*
**	SendTextKeyStroke()
**
**	handle a keystroke
*/
bool
SendTextKeyStroke( int ch, uint modifiers )
{
	bool bResult = false;
	
	// Check to see if we need to stop any executing macros	
	StopMacrosOnKey();
	
	if ( ch )
		{
		if ( modifiers & kKeyModMenu )
			bResult = DoSpecialKey( ch );
		else
			bResult = DoNormalKey( ch, modifiers );
		}
	
	return bResult;
}


namespace
{

/*
**	DoSpecialKey()
**
**	handle a keystroke with the menu key modifier held down
*/
bool
DoSpecialKey( int ch )
{
	bool bResult = true;
	
	switch ( ch )
		{
		case kUpArrowKey:
			UseCachedString( +1 );
			break;
		
		case kDownArrowKey:
			UseCachedString( -1 );
			break;
		
		case kRightArrowKey:
			gSendText.Insert( "\r" );
			break;
		
		default:
			bResult = false;
			break;
		}
	
	return bResult;
}


/*
**	UseCachedString()
**
**	copy a string out of the cache
*/
void
UseCachedString( int delta )
{
	static char cmdStr[ kPlayerInputStrLen ];
	
	int direction = delta < 1 ? CTextCmdList::cmd_Last : CTextCmdList::cmd_First;
	
	gSendText.GetText( cmdStr, sizeof cmdStr );
	if ( gCmdList->GetCachedCmd( direction, cmdStr ) )
		{
		gSendText.SetText( cmdStr );
		gSendText.SelectText( 0, 0x7FFF );
		gSendText.Draw();
		}
}


/*
**	DoNormalKey()
**
**	do the correct thing to the text field
*/
bool
DoNormalKey( int ch, uint modifiers )
{
	// expand replacement macros
	DoMacroReplacement( ch, modifiers );
	
	switch ( ch )
		{
		// prepare to send the string to the server
		case kReturnKey:
		case kEnterKey:
			DoEnterKey();
			break;
		
		// clear the edit text
		case kEscapeKey:
			gSendText.Clear();
			break;
		
		// select all the text in the field
		case '\t':
			gSendText.SelectText( 0, 0x7FFF );
			break;
		
		// send the character to the edit field
		default:
			gSendText.KeyStroke( ch, modifiers );
			break;
		}
	
	return true;
}


/*
**	DoEnterKey()
**
**	handle when the enter key is hit
*/
void
DoEnterKey()
{
	static char cmdStr[ kPlayerInputStrLen ];
	gSendText.GetText( cmdStr, sizeof cmdStr );
	
	//
	// a bit of debug code
	//
#ifdef DEBUG_VERSION
	if ( 0 == std::strcmp( cmdStr, BULLET BULLET "report" ) )
		{
		gCmdList->DebugReport();
		}
	else
#endif
	
	// See if the macro expander handles it
	if ( DoMacroText( cmdStr ) )
		{
		gSendText.SelectText( 0, 0x7FFF );
		return;
		}
	
	if ( gCmdList->IsSendCmdAvail() )
		{
		// gSendText.SetText( cmdStr );
		gSendText.SelectText( 0, 0x7FFF );
		gCmdList->SendCmd( true, cmdStr );
		
#if ! CLIENT_COMMANDS
		// rer v79: optionally stick commands into textlog
		// v85: except if they'd ordinarily show up there anyway
		if ( gPlayingGame
		&&	 ( '/' == cmdStr[0] || '\\' == cmdStr[0] ) )
			{
			LogTextCmd( cmdStr );
			}
		// else Commands_cl.cp handles logging ones that need it.
#endif
		}
	else
		Beep();
}


#if ! CLIENT_COMMANDS
/*
**	LogTextCmd()
**
**	copy commands to log & file, filtering out non-essentials
*/
void
LogTextCmd( const char * cmd )
{
	char buffer[ 16 ];
	
	// force lowercase for comparison (skipping leading / or \ )
	// we only look at the first 15 chars; anything longer surely is not a command
	StringCopySafe( buffer, cmd + 1, sizeof buffer );
	StringToLower( buffer );
	
	// filter out commands that don't need logging
	if ( ( std::strcmp( buffer, "whisper" ) == 0 )
	||	 ( std::strcmp( buffer, "yell" ) == 0 )
	||	 ( std::strcmp( buffer, "growl" ) == 0 )
	||	 ( std::strcmp( buffer, "ponder" ) == 0 )
	||	 ( std::strcmp( buffer, "action" ) == 0 )
	||	 ( std::strcmp( buffer, "think" ) == 0 && std::strcmp( buffer, "thinkto" ) != 0 ) )
		{
		return;
		}
	
	// let's put a '>' prompt for good measure
	char logged[ 256 ];
	std::snprintf( logged, sizeof logged, ">%s", cmd );
	AppendTextWindow( logged );
}
#endif  // ! CLIENT_COMMANDS

}	// namespace


/*
**	SendTextCmd()
**
**	handle a menu command
*/
bool
SendTextCmd( long menuCmd )
{
	bool result = true;
	switch ( menuCmd )
		{
		case mUndo:
			gSendText.Undo();
			break;
		
		case mCut:
			gSendText.Copy();
			gSendText.Clear();
			break;
		
		case mCopy:
			gSendText.Copy();
			break;
		
		case mPaste:
			gSendText.Paste();
			break;
		
		case mClear:
			gSendText.Clear();
			break;
		
		case mSelectAll:
			gSendText.SelectText( 0, 0x7FFF );
			break;

		default:
			result = false;
			break;
		}
	
	return result;
}


/*
**	SendText()
**
**	Generic function to queue some text (command, speaking).
*/
bool
SendText( const char * inTextToSend, bool quiet )
{
	if ( gCmdList->IsSendCmdAvail() )
		return gCmdList->SendCmd( not quiet, inTextToSend );
	
	Beep();
	return false;
}


/*
**	SendCommand()
**
**	send a simple command to the server.
*/
bool
SendCommand( const char * cmdName, const char * cmdParam, bool quiet )
{
	static char cmdStr[ kPlayerInputStrLen ];
	
	if ( not cmdParam
	||	 '\0' == cmdParam[ 0 ] )
		snprintf( cmdStr, sizeof cmdStr, "\\%s", cmdName );
	else
		snprintf( cmdStr, sizeof cmdStr, "\\%s \"%s\"", cmdName, cmdParam );
	
	return SendText( cmdStr, quiet );
}


/*
**	InsertName()
**
**	Insert a player name into the TextEdit field.
**	Remove non-alphas, and surround with spaces fore & aft.
**
*/
void
InsertName( const char * name )
{
	// easy out?
	if ( not name )
		return;
	
	int pos1 = 0;
	int pos2 = 0;
	
	char tmpName[ kMaxNameLen + 2 ];
	tmpName[ pos2++ ] = ' ';
	
	char tc;
	while ( (tc = name[pos1++]) != '\0' )
		{
		if ( (tc & 0xDF) < 'A'
		||	 (tc & 0xDF) > 'Z' )
			{
			continue;
			}
		tmpName[ pos2++ ] = tc;
		}
	
	tmpName[ pos2++ ] = ' ';
	tmpName[ pos2++ ] = '\0';
	
	gSendText.Clear();
	gSendText.Insert( tmpName );
}


/*
**	SendTextField::DoServiceEvent()
**	handle OS X services negotiation
*/
OSStatus
SendTextField::DoServiceEvent( const CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	OSStatus err;
	ScrapRef serviceScrap = nullptr;
	char * buff = nullptr;
	
	// cache the current selection limits
	int start, stop;
	SInt32 length;
	GetSelect( &start, &stop );
	
	// CFStrings for the OSTypes of the kinds of data we can copy or paste
	static CFStringRef sTypeMacText;
	if ( not sTypeMacText )
		{
		sTypeMacText = CreateTypeStringWithOSType( kScrapFlavorTypeText );		// 'TEXT'
		__Check( sTypeMacText );
		}
	
	// enabler flags for handling Unicode text in OSX services
#define CAN_COPY_UNICODE		1
	
#if CAN_COPY_UNICODE
	static CFStringRef sTypeUnicode;
	if ( not sTypeUnicode )
		{
		sTypeUnicode = CreateTypeStringWithOSType( kScrapFlavorTypeUnicode );	// 'utxt'
		__Check( sTypeUnicode );
		}
#endif
	
	switch ( evt.Kind() )
		{
		case kEventServiceGetTypes:
			{
			// we can only handle text (maybe unicode too later)
			// ...which we can basically always paste
			
			//if ( noErr == err )
				{
				CFMutableArrayRef pasteTypes;
				err = evt.GetParameter( kEventParamServicePasteTypes,
							typeCFMutableArrayRef, sizeof pasteTypes, &pasteTypes );
				__Check_noErr( err );
				if ( noErr == err )
					{
//					CFArrayAppendValue( pasteTypes, sTypeUnicode );	// not yet
					CFArrayAppendValue( pasteTypes, sTypeMacText );
					}
				}
			
			// is there a selection? if not, we have nothing to copy
			if ( start < stop )
				{
				CFMutableArrayRef copyTypes;
				err = evt.GetParameter( kEventParamServiceCopyTypes,
						typeCFMutableArrayRef, sizeof copyTypes, &copyTypes );
				__Check_noErr( err );
				if ( noErr == err )
					{
#if CAN_COPY_UNICODE
					CFArrayAppendValue( copyTypes, sTypeUnicode );
#endif
					CFArrayAppendValue( copyTypes, sTypeMacText );
					}
				}
			
			result = err;
			} break;
		
		case kEventServiceCopy:
			{
			// can't copy if there's no selection
			if ( start >= stop )
				break;
			
			length = GetTextLength();
			
			err = evt.GetParameter( kEventParamScrapRef,
						typeScrapRef, sizeof serviceScrap, &serviceScrap );
			__Check_noErr( err );
			
			if ( noErr == err )
				{
				// get entire contents of text field into a buffer...
				buff = NEW_TAG( "ServiceCopy" ) char[ length + 1 ];
				__Check( buff );
				if ( not buff )
					{
					err = memFullErr;
					break;
					}
				GetText( buff, length + 1 );
				
				// start with a clear conscience
				__Verify_noErr( ClearScrap( &serviceScrap ) );
				
				// but only put the selected subsection thereof onto the scrap
				err = PutScrapFlavor( serviceScrap, kScrapFlavorTypeText, kScrapFlavorMaskNone,
								stop - start, buff + start );
				__Check_noErr( err );
				
				// also as Unicode
#if CAN_COPY_UNICODE
				CFStringRef cs = CFStringCreateWithBytes( kCFAllocatorDefault,
									(UInt8 *) buff + start, stop - start,
									kCFStringEncodingMacRoman, false );
				if ( cs )
					{
					CFIndex curLen = CFStringGetLength( cs );
					UniChar * ubuf = NEW_TAG( "ServiceCopyUTF" ) UniChar[ curLen ];
					if ( ubuf )
						{
						CFStringGetCharacters( cs, CFRangeMake(0, curLen), ubuf );
						
						err = PutScrapFlavor( serviceScrap,
									kScrapFlavorTypeUnicode, kScrapFlavorMaskNone,
									curLen * sizeof(UniChar), ubuf );
						__Check_noErr( err );
						
						delete[] ubuf;
						}
					CFRelease( cs );
					}
#endif  // CAN_COPY_UNICODE
				
				delete[] buff;
				result = err;
				}
			}
			break;
		
		case kEventServicePaste:
			err = evt.GetParameter( kEventParamScrapRef,
						typeScrapRef, sizeof serviceScrap, &serviceScrap );
			__Check_noErr( err );
			if ( noErr == err )
				{
				err = GetScrapFlavorSize( serviceScrap, kScrapFlavorTypeText, &length );
				__Check_noErr( err );
				if ( noErr == err )
					{
					buff = NEW_TAG( "ServicePaste" ) char[ length + 1 ];
					__Check( buff );
					if ( not buff )
						err = memFullErr;
					else
						{
						err = GetScrapFlavorData( serviceScrap, kScrapFlavorTypeText,
							&length, buff );
						__Check_noErr( err );
						if ( noErr == err )
							{
							buff[ length ] = '\0';	// null-terminate, just in case
							Insert( buff );
							}
						}
					delete[] buff;
					}
				}
			result = err;
			break;
		}
	
	return result;
}


/*
**	SendTextField::HandleEvent()
**	so far, we only handle service events
*/
OSStatus
SendTextField::HandleEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	// handle service events
	if ( kEventClassService == evt.Class() )
		result = DoServiceEvent( evt );
	
	return result;
}

#pragma mark -


namespace {

/*
**	HandleDoSendTextAEvt()
**
**	Accept a string of text from an AppleScript, then
**	proceed as if the user typed it himself.
*/
OSErr
HandleDoSendTextAEvt( const AppleEvent * evt, AppleEvent * reply, SRefCon /*refcon*/ )
{
	// are we connected?
	if ( gPlayingGame && gNetworking && not CCLMovie::IsReading() )
		{
		char theText[ kPlayerInputStrLen ];
		DescType realType;
		SInt32 realSize;
		
		// extract text from evt
		OSStatus result = AEGetParamPtr( evt, keyDirectObject, typeText,
							&realType, &theText, kPlayerInputStrLen, &realSize );
		if ( noErr == result && realSize > 0 )
			{
			// don't overflow the buffer
			if ( realSize >= kPlayerInputStrLen )
				realSize = kPlayerInputStrLen - 1;
			
			// C strings need trailing nulls
			theText[ realSize ] = '\0';
			
			// push the text through the macro expander...
			if ( not DoMacroText( theText ) )
				{
				// ... and have the game process it if the macro expander didn't
				SendText( theText, true );
				}
			}
		
		return result;
		}
	
	// We weren't connected, so we can hardly be expected to handle this request.
	// "You are not connected to the game."
	const char * const errMsg = TXTCL_NOT_CONNECTED;
	(void) AEPutParamPtr( reply, keyErrorString, typeUTF8Text, errMsg, strlen(errMsg) );
	return errAEEventNotHandled;
}


/*
**	InstallTextHandler()
**
**	tell OS about our event handler for the Do Script AppleEvent
*/
DTSError
InstallTextHandler()
{
	DTSError result = noErr;
	static AEEventHandlerUPP theUPP;
	if ( not theUPP )
		{
		theUPP = NewAEEventHandlerUPP( HandleDoSendTextAEvt );
		result = AddAppleEventHandler( kAEMiscStandards, kAEDoScript, 0, theUPP );
		}
	return result;
}

}	// anon namespace


