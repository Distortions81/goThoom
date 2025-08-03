/*
**	LaunchURL_cl.cp		ClanLord
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
#include "LaunchURL_cl.h"


/*
**	local definitions
*/
const Style kLaunchURL_LinkFace = 0x80;		// an unused value in the
											// {normal,bold,italic...} range


/*
**	global variables
*/
LaunchURLHandler	gLaunchURLHandler;
URLMenu				gURLMenu;


/*
**	LaunchURLHandler::Init()
**
**	Sees which URL schemes are supported
*/
OSStatus
LaunchURLHandler::Init()
{
	// Initialize these fields in case we bail on an error
	// 'https' must come before 'http' otherwise
	// it'll never be recognized separately
	strcpy( helpers[ kLaunchHandler_HTTPSHelper  ].scheme, "https"  );
	strcpy( helpers[ kLaunchHandler_HTTPHelper   ].scheme, "http"   );
	strcpy( helpers[ kLaunchHandler_MAILTOHelper ].scheme, "mailto" );
	strcpy( helpers[ kLaunchHandler_FTPHelper    ].scheme, "ftp"    );
	
	for ( int i = 0;  i < kLaunchHandler_NumHelpers;  ++i )
		{
		helpers[ i ].available = false;		// assume the worst
		
		char buff[ 256 ];
		
		// the actual URL values here are meaningless; we're only interested
		// in whether LaunchServices can handle a particular _scheme_.
		int len;
		if ( kLaunchHandler_MAILTOHelper != i )
			len = snprintf( buff, sizeof buff,
					"%s://www.deltatao.com/", helpers[i].scheme );
		else
			{
			// But we still need to special-case the email one, 
			// since <mailto://www.deltatao.com/> is malformed (no "@").
			len = snprintf( buff, sizeof buff,
					"%s:joe@deltatao.com", helpers[i].scheme );
			}
		
		if ( CFURLRef url = CFURLCreateWithBytes( kCFAllocatorDefault,
							(uchar *) buff, len,
							kCFStringEncodingASCII, nullptr ) )
			{
#if DEBUG_URL_INIT
			CFURLRef appURL = nullptr;
			OSStatus err = LSGetApplicationForURL( url,
								kLSRolesAll, nullptr, &appURL );
			if ( appURL )
				{
				CFShow( appURL );
				// must release even tho' it came from a "Get" function:
				// see <LSInfo.h>
				CFRelease( appURL );
				}
#else
			OSStatus err = LSGetApplicationForURL( url,
							kLSRolesAll, nullptr, nullptr );
#endif  // DEBUG_URL_INIT
			
			helpers[i].available = (noErr == err);
			
			CFRelease( url );
			}
		}
	
	return noErr;
}


/*
**	LaunchURLHandler::LaunchURL
**
**	Launches the URL
*/
OSStatus
LaunchURLHandler::LaunchURL( const char * text, long * urlStart, long * urlEnd )
{
	// Copy the url for error messages
	char url[ 256 ];
	size_t len = *urlEnd - *urlStart > int(sizeof(url) - 1) ?
					sizeof(url) - 1 : size_t(*urlEnd - *urlStart);
	BufferToStringSafe( url, sizeof url, text + *urlStart, len );
	
#if 0  // always available on X
	if ( not Available() )
		{
		SafeString errMessage;
		errMessage.Format(
				_(TXTCL_URL_NO_INTERNETCONFIG),
/*				"You do not have \"Internet Config\" installed or there was "
				"insufficient memory for it to load when Clan Lord started. "
				"This system extension is required for clickable links to work.\r\r"
				" %s", */
				url );
		GenericError( "%s", errMessage.Get() );
		return kLaunchHandlerError_ICNotAvailable;
		}
#endif  // 0
	
	bool protocolString = false;
	int i;
	// Compare the first characters to our various protocols ignoring case
	for ( i = 0;  i < kLaunchHandler_NumHelpers;  ++i )
		{
		if ( 0 == strncasecmp( helpers[i].scheme, text + *urlStart,
								strlen( helpers[ i ].scheme ) ) )
			{
			protocolString = true;
			break;
			}
		}
	
	// If we didn't find one, assume it is an e-mail
	if ( i >= kLaunchHandler_NumHelpers )
		i = kLaunchHandler_MAILTOHelper;
	
	// Check to see if this protocol is enabled - return a message if it isn't
	if ( not helpers[i].available )
		{
		// FIXME: this error message is sadly out of date.
		SafeString errMessage;
		errMessage.Format(
			_(TXTCL_URL_NO_HELPERAPP),
/*			"You do not have a helper application choosen for the type '%s'.\r\r"
			"Use \"Internet Config\" to choose one and restart Clan Lord if you "
			"would like this link to work.\r\r%s", */
			helpers[ i ].scheme,
			url );
		GenericError( "%s", errMessage.Get() );
		
		return kLaunchHandlerError_ICNotAvailable;
		}
	
	OSStatus err = noErr;
	
	// cons up a complete URL string: ie., prepend explicit 'mailto:' if needed
	char url2[ 256 ];
	if ( protocolString )
		StringCopySafe( url2, url, sizeof url2 );
	else
		snprintf( url2, sizeof url2, "mailto:%s", url );
	
	// Make URL object, open it, enjoy!
	if ( CFURLRef u = CFURLCreateWithBytes( kCFAllocatorDefault,
							(uchar *) url2, strlen(url2),
							kCFStringEncodingMacRoman, nullptr ) )
		{
		err = LSOpenCFURLRef( u, nullptr );
		CFRelease( u );
		}
	
	return err;
}


/*
**	FindProtocol
**
**	Looks for something starting with a "protocol:" string (aka "scheme")
*/
static const char *
FindProtocol( const char * myFindIn, const char * schemeToFind )
{
	const char * temp = myFindIn;
	const char * mFound = strcasestr( temp, schemeToFind );
	while ( mFound )
		{
		// If it is the first character, or if the character before
		// is in our "break" string
		if ( mFound - 1 < myFindIn
		||   strchr( kLaunchURLHandler_Breaks, *(mFound - 1) ) )
			{
			// If there isn't an alphanumeric character before
			// the next break character
			const uchar * end = reinterpret_cast<const uchar *>( mFound )
							  + strlen( schemeToFind );
			
			// If the next character isn't a colon
			if ( ':' != *end )
				return nullptr;
			++end;
			
			// search for the end of the protocol
			while ( not strchr( kLaunchURLHandler_Breaks, *end )
			&&      not isalnum( *end ) )
				{
				++end;
				}
			if ( isalnum( *end ) )
				return mFound;
			}
		
		temp = mFound + 1;
		mFound = strcasestr( temp, schemeToFind );
		}
	
	return nullptr;
}


/*
**	FindEmail
**
**	Looks for something of the form N@N.N 
*/
static const char *
FindEmail( const char * text )
{
	const char * temp = text;
	const char * end = text + strlen( text );
	const char * middle = strchr( text, '@' );
	
	while ( middle )
		{
		//	Look backwards for the first break
		const char * begin = middle;
		while ( not strchr( kLaunchURLHandler_Breaks, *begin ) && begin >= text )
			{
			--begin;
			}
		++begin;
		
		// If we didn't find one, it will be set to the beginning of the text
		// Check for an alphanumberic to start it
		if ( isalnum( *begin ) )
			{
			const char * finish = middle + 1;
			// Search forwards for a period that has at least one character after it			
			while ( ( '.' != *finish ) && ( finish < end - 2 ) )
				{
				++finish;
				}
			
			if ( '.' == *finish )
				{
				// Make sure what is to both sides of the period is alphanumeric
				if ( isalnum( *(finish - 1) ) && isalnum( *(finish + 1) ) )
					return begin;
				}
			}
		
		// Go past the mark and find the next @ if this one was bad
		temp = middle + 1;
		middle = strchr( temp, '@' );
		}
	
	return nullptr;
}


/*
**	LaunchURLHandler::ParseForURL
**
**	Parses text for a URL
*/
OSStatus
LaunchURLHandler::ParseForURL( const char * data,
							   long * urlStart, long * urlEnd ) const
{
	// Parse it out even if the URL's scheme isn't currently enabled;
	// the error code tells us we need to draw it disabled
	
	if ( *urlStart >= *urlEnd )
		return kLaunchHandlerError_ZeroSelection;
	
	const char * findIn = data + * urlStart;
	const char * found, * earlierFound;
	const char * end = data + strlen( data );
	int protocolIndex = -1;
	
	found = end;
	
	OSStatus result = kLaunchHandlerError_NoURLFound;
	
	// Search for the scheme: strings
	for ( int i = 0;  i < kLaunchHandler_NumHelpers;  ++i )
		{
		earlierFound = FindProtocol( findIn, helpers[ i ].scheme );
		if ( earlierFound && (earlierFound < found) )
			{
			found = earlierFound;
			protocolIndex = i;
			}
		}
	
	// Also search for something that looks like an e-mail
	earlierFound = FindEmail( findIn );
	if ( earlierFound && ( earlierFound < found ) )
		{
		found = earlierFound;
		protocolIndex = kLaunchHandler_MAILTOHelper;
		}
	
	// Found it and it was at an index less than the end
	if ( (found != end) && ( found - data <= *urlEnd ) )
		{
		*urlStart = found - data;
		found = strpbrk( found, kLaunchURLHandler_Breaks );
		
		if ( not found )
			*urlEnd = strlen( data );
		else
			*urlEnd = found - data;
		
		// Go backwards removing ending non-alphanumerics except '/'
		while ( (*urlEnd > *urlStart) && ( not isalnum( *(data + *urlEnd - 1)) )
				&& ('/' != *(data + *urlEnd - 1) ) )
			{
			--(*urlEnd);
			}
		
		//	Found one! --  but is it available?
		if ( helpers[ protocolIndex ].available )
			result = noErr;
		else
			result = kLaunchHandlerError_ICNotAvailable;
		}
	
	return result;
}

#pragma mark -


/*
**	URLMenu::DoCmd()
**
**	open a URL which is listed in the help menu
*/
bool
URLMenu::DoCmd( int menuCmd )
{
#ifdef ARINDAL
# define HELP_URL	CFSTR("http://www.arindal.com/")
#else
# define HELP_URL	CFSTR("https://www.deltatao.com/clanlord/index.html")
#endif
	
	// bail if this command isn't from the help menu at all
	if ( kHMHelpMenuID != HiWord( menuCmd ) )
		return false;
	
	// help menu items are numbered starting with 1
	if ( 1 == LoWord( menuCmd ) )
		{
		if ( CFURLRef url = CFURLCreateWithString( kCFAllocatorDefault,
								HELP_URL, nullptr ) )
			{
			LSOpenCFURLRef( url, nullptr );
			CFRelease( url );
			}
		
		// we handled the command
		return true;
		}
	
	return false;
}

#pragma mark -


/*
**	IsLink()
**
**	Checks the unused bit of the face of the text
**	if it's set, then this is a link
*/
bool 
URLStyledTextField::IsLink( const DTSPoint& loc ) const
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return false;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return false;
	
	// Get the style and offset of the character to the left of the clicked one
	TextStyle theTS;
	Point where = * DTS2Mac( &loc );
	int offset = ::TEGetOffset( where, hTE ) - 1;
	if ( offset < 0 )
		offset = 0;
	
	SInt16 junk1, junk2;
	::TEGetStyle( offset, &theTS, &junk1, &junk2, hTE );
	return 0 != (theTS.tsFace & kLaunchURL_LinkFace);
}


/*
**	URLStyledTextField::StyleTextURL()
**
**	Underlines a URL and sets a disabled or enabled color
**	over the current text face
**	Note: setting the unused bit in the face field of the text style
**	tells it to treat this as a URL.
*/
void
URLStyledTextField::StyleTextURL( bool enabled )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
	// Set the color for the entire selection
	TextStyle theTS;
	if ( enabled )
		{
		// Blue
		theTS.tsColor.red   =
		theTS.tsColor.green = 0;
		theTS.tsColor.blue  = 0xFFFF;
		}
	else
		{
		// Grey
		theTS.tsColor.red   =
		theTS.tsColor.blue  =
		theTS.tsColor.green = 0x8888;
		}
	::TESetStyle( doColor, &theTS, true, hTE );
	
	int oldStart, oldEnd;
	GetSelect( &oldStart, &oldEnd );
	
	// Set underlining only for the last character
	SelectText( oldEnd - 1, oldEnd );
	theTS.tsFace = underline;
	::TESetStyle( doFace, &theTS, true, hTE );
	
	// Underline and mark as a link for the rest
	SelectText( oldStart, oldEnd - 1 );
	theTS.tsFace = kLaunchURL_LinkFace | underline;
	::TESetStyle( doFace, &theTS, true, hTE );
	
	//	Not having the last character count as a link prevents the
	//	style from continuing for the rest of the window if this is
	//	the last character
	
	//	Looking at this face bit of the character to the left of the
	//	one the mouse is on tells us if we should treat the mouse like
	//	it is on a link
	
	SelectText( oldStart, oldEnd );
}


/*
**	URLStyledTextField::StyleInsert
**
**	Styles links within the inserted text
*/
void
URLStyledTextField::StyleInsert( const char * text, const CLStyleRecord * style )
{
	int selEnd, selStart;
	GetSelect( &selStart, &selEnd );
	
	// Insert the text...
	StyledTextField::StyleInsert( text, style );
	
	// ...  and then look for URLs in it and style these over the default style
	int newEnd, newStart;
	GetSelect( &newStart, &newEnd );
	
	long parseBegin = 0;
	long parseEnd = strlen( text );
	long parseTempEnd = parseEnd;
	
	OSStatus err = gLaunchURLHandler.ParseForURL( text, &parseBegin, &parseTempEnd );
	
	while ( noErr == err
	||		kLaunchHandlerError_ICNotAvailable == err )
		{
		SelectText( selStart + parseBegin, selStart + parseTempEnd );
		
		StyleTextURL( kLaunchHandlerError_ICNotAvailable != err );
		
		parseBegin = parseTempEnd + 1;
		parseTempEnd = parseEnd;
		
		err = gLaunchURLHandler.ParseForURL( text, &parseBegin, &parseTempEnd );
		}
	
	SelectText( newStart, newEnd );
}


/*
**	URLStyledTextField::Click
**
**	Launches URLs when clicked
*/
bool
URLStyledTextField::Click( const DTSPoint& loc, ulong time, uint modifiers )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return false;
	
	// safety first
	TEHandle hTE = p->textTEHdl;
	Handle hTEText = (*hTE)->hText;
	
	// cache the boundaries
	const char * hTextBegin = *hTEText;
	const char * teTextEnd	= hTextBegin + ::GetHandleSize( hTEText );
	
	// If the click is in a point that is formatted like a link,
	// then read in the link and launch it
	if ( IsLink( loc ) )
		{
		int offset = ::TEGetOffset( *DTS2Mac(&loc), hTE );
		
		// Search backwards and forwards for the first position that is in
		// our string of break characters.
		const char * start = hTextBegin + offset;
		const char * end = start;
		
		// also don't permit "hi-ascii" chars
		while ( not strchr( kLaunchURLHandler_Breaks, *start )
		 &&		* (const uchar *) start < 0x7F
		 &&		start >= hTextBegin )
			{
			--start;
			}
		
		++start;
		
		if ( start < hTextBegin + offset )
			{
			while ( not strchr( kLaunchURLHandler_Breaks, *end )
			&&		* (const uchar *) end < 0x7F
			&&		end < teTextEnd )
				{
				++end;
				}
			//--end;
			
			long selStart = start - hTextBegin;
			long selEnd = end - hTextBegin;
			
			// Set the mouse to an arrow in case we show the generic alert
			// from inside this function
			DTSShowArrowCursor();
			
			gLaunchURLHandler.LaunchURL( hTextBegin, &selStart, &selEnd );
			
			// Don't handle the click further or we get insertion within a URL
			return true;
			}
		}
	
	return StyledTextField::Click( loc, time, modifiers );
}


/*
**	URLStyledTextField::Idle()
**
**	Changes the cursor to a pointer when over a URL
*/	
void
URLStyledTextField::Idle()
{
	StyledTextField::Idle();
	
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TEHandle hTE = p->textTEHdl;
	
	DTSWindow * win = gDTSApp->GetFrontWindow();
	DTSPoint where;
	win->GetMouseLoc( &where );
	
	if ( where.InRect( reinterpret_cast<const DTSRect *>( &((*hTE)->viewRect)) ) )
		{
		if ( IsLink( where ) )
			gCursor = kLaunchURLHandler_PointCursorID;
		}
}
