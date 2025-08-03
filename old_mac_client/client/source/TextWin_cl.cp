/*
**	TextWin_cl.cp		ClanLord
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
#include "Movie_cl.h"
#if USE_STYLED_TEXT
# include "LaunchURL_cl.h"
#endif
#include "Macros_cl.h"
#include "Dialog_mach.h"
#include "GameTickler.h"


/*
**	Entry Routines
*/
/*
void	CreateTextWindow( const DTSRect * pos );
void	StartTextLog();
void	CloseTextLog();
void	AppendTextWindow( const char * );
void	AppendTextWindow( const char * text, const CLStyleRecord * );	// new overload
*/


/*
**	Definitions
*/
#define	kTextLogsFolder			"Text Logs"
#define	kTextLogsMovieSubfolder	"CL_Movies"
	// should be an invalid character (i.e. player) name...
	// e.g. include a prohibited character (i.e. byte), such as "_"


/*
**	TextWindow class
*/
class TextScrollBar : public DTSControl
{
public:
	// overloaded routines
	virtual void	Hit();
};


/*
**	The ScrollTextField class exists merely to provide access to TextEdit's
**	clickLoop callback, which we need to use in order to synchronize the
**	scrolling of the textfield (during a mousedown) with the position of
**	the scroll bar.
*/
class ScrollTextField : public TextFieldType
{
#if ! TEXTFIELD_USES_MLTE
public:
	// trivial constructor
							ScrollTextField() :	prevClickLoop( nullptr ) {}
	
	// interface
	virtual bool			Click( const DTSPoint& loc, ulong time, uint mods );
	
protected:
	// the real click loop function
	bool					MyClickLoop( TEPtr pTE );
	
	// glue code to mediate between the OS and the C++ object
	static Boolean			DoClickLoop( TEPtr pTE );
	
	// prevClickLoop provides a 1-deep stack for daisy-chaining hook procs
	TEClickLoopUPP			prevClickLoop;
#endif  // ! TEXTFIELD_USES_MLTE
};


class TextWindow : public DTSWindow, public CarbonEventResponder
{
public:
	TextScrollBar		twBar;
	ScrollTextField		twText;
	bool				twActive;
	bool				twSearchWraps;
	bool				twIgnoreCase;
	int 				theOffset;
	char				searchstring[ 256 ];
	
	// constructor
						TextWindow();
	
	// overloaded routines
	virtual void		DoDraw();
	virtual void		Activate();
	virtual void		Deactivate();
	virtual void		Size( DTSCoord h, DTSCoord v );
	virtual void		Idle();
	virtual bool		Click( const DTSPoint& loc, ulong time, uint modifiers );
	virtual bool		KeyStroke( int ch, uint modifiers );
	virtual bool		DoCmd( long menuCmd );
	
	// public interface
	void				SetRange();
	int					GetRange() const;
//	int					GetNumLines() const;
	void				KillOldText();
	void				DoFind();
	void				DoFindAgain();
	OSStatus			InstallEvents();
	
	static void			GetMetrics();
	
	// internals
protected:
	void				HiliteSelection();
	
	enum
		{
		kTextMaxLength	= 30 * 1000,
		kTextMaxLenKill	= kTextMaxLength - 2000
		};
	
		// carbon events
	OSStatus			DoScrollEvent( const CarbonEvent& );
	OSStatus			DoServiceEvent( const CarbonEvent& );
	OSStatus			DoWindowEvent( const CarbonEvent& );
	virtual OSStatus	HandleEvent( CarbonEvent& );
	
		// theme metrics
	static SInt32	sBarWd;
	static SInt32	sBarInset;
	static SInt32	sGrowHt;
};


/*
**	FindDlg class
*/
class FindDlg : public HIDialog, public GameTickler
{
	static const OSType Signature = 'Find';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef GetNibFileName() const { return CFSTR("Find Text"); }
	virtual CFStringRef GetPositionPrefKey() const { return CFSTR("FindTextDlgPosition"); }
	
	static const int	kFindTextSize = 256;
	
	char				findText[ kFindTextSize ];
	bool				fdSearchWraps;
	bool				fdIgnoreCase;
	
	// overloaded routines
	virtual void		Init();
	
	enum
		{
		fdFindTextEdit	= 1,
		fdSearchWrapsCheck = 2,
		fdIgnoreCaseCheck = 3
		};
	
public:
	static bool		DoFindTextDialog( char * searchstring, bool& searchWraps, bool& bIgnoreCase );
};


/*
**	Internal Variables
*/
static FILE *	gTextFile;

SInt32			TextWindow::sBarWd;
SInt32			TextWindow::sBarInset;
SInt32			TextWindow::sGrowHt;


/*
**	SwapLineEndings()
**
**	exchange \r's and \n's
*/
static void
SwapLineEndings( char * inText )
{
	uchar * text = reinterpret_cast<uchar *>( inText );
	while ( *text )
		{
		if ( '\n' == *text )
			*text = '\r';
		else
		if ( '\r' == *text )
			*text = '\n';
		
		++text;
		}
}

#pragma mark -


/*
**	CreateTextWindow()
**
**	Create and show the text transcript window.
*/
void
CreateTextWindow( const DTSRect * pos )
{
	// create the window
	TextWindow * win = NEW_TAG("TextWindow") TextWindow;
	CheckPointer( win );
	assert( win );
	gTextWindow = win;
	
	// create window of type document with close, zoom, and grow boxes.
	DTSError result = win->Init( kWKindDocument |
									kWKindCloseBox | kWKindZoomBox | kWKindGrowBox |
									kWKindLiveResize );
	if ( result )
		return;
	
	win->GetMetrics();
	
	// create the scroll bar
	win->twBar.Init( kControlVertScrollBar, "", win );
	CheckError();
	win->twBar.Show();
	
	// create the text field
	DTSRect box;
	box.Set( kGameInputMargin, kGameInputMargin, 100, 100 );
	
#if TEXTFIELD_USES_MLTE
	result = win->twText.Init( win, &box, nullptr, 0 );
#else
	// choose the default application font
	char fontName[ 256 ];
	SInt16 fSize;
		{
		Str255 fontNm;
		Style fStyle;	// ignored
		__Verify_noErr( GetThemeFont( kThemeApplicationFont, smRoman,
							fontNm, &fSize, &fStyle ) );
		CopyPascalStringToC( fontNm, fontName );
		}
	result = win->twText.Init( win, &box, fontName, fSize );
#endif
	if ( noErr != result )
		return;
	
	win->twActive = false;
//	win->twText.SetReadOnly( true );
	win->twText.Show();
	
	// set the title and position it
	win->SetTitle( _(TXTCL_TITLE_TEXT) );		// "Text"
	int top  = pos->rectTop;
	int left = pos->rectLeft;
	win->Move( left, top );
	win->Size( pos->rectRight - left, pos->rectBottom - top );
	win->Show();
	
	win->InstallEvents();
	
	// semi-fancy text rendering
	QDSwapPortTextFlags( ::GetWindowPort( DTS2Mac(win) ), kQDSupportedFlags );
}


/*
**	AppendTextWindow()
**
**	add text to the text window
*/
void AppendTextWindow( const char * text )
{
	CLStyleRecord style;
	style.font = applFont;
	style.face = normal;
	style.size = 12;
	style.color = DTSColor::black;
	
	AppendTextWindow( text, &style );
}


/*
**	AppendTextWindow()
**
**	add text to the text window, with style info
*/
void
AppendTextWindow( const char * text, const CLStyleRecord * style )
{
	// inform macro unit of this most recent line
	SetMacroTextWinBuffer( text );
	
	// paranoid check
	TextWindow * win = static_cast<TextWindow *>( gTextWindow );
	if ( not win )
		return;
	
	// hide the thing so the gymnastics we do
	// don't affect the user
	__Verify_noErr( DisableScreenUpdates() );
	
	// save the top line
	int topLine = win->twText.GetTopLine();
	int maxLine = win->twBar.GetMax();
	bool atBottom = win->twText.AtBottom();
	
	// make sure we don't have too much text
	int linesKilled = win->twText.GetNumLines();
	win->KillOldText();
	linesKilled -= win->twText.GetNumLines();
	
	// save the selection
	// preserve selections that are at the end
	int start, stop;
	win->twText.GetSelect( &start, &stop );
	int len = win->twText.GetTextLength();
	if ( start == len )
		{
		start = 0x7FFF;
		stop  = 0x7FFF;
		}
	
	// move the insertion point to the end
	win->twText.SelectText( 0x7FFF, 0x7FFF );
	if ( len > 0 )
		win->twText.Insert( "\r" );
	
	std::FILE * stream = gTextFile;
	if ( stream )
		fputc( '\n', stream );
	
	// add the time stamp
	if ( gPrefsData.pdTimeStamp )
		{
		char buffer[ 32 ];
		DTSDate date;
		date.Get();
		int hour = date.dateHour;
		char ampm = ( hour >= 12 ) ? 'p' : 'a';
		hour = ( hour + 11 ) % 12 + 1;
		snprintf( buffer, sizeof buffer, "%d/%d/%.2d %d:%.2d:%.2d%c ",
			date.dateMonth,
			date.dateDay,
			date.dateYear % 100,
			hour,
			date.dateMinute,
			date.dateSecond,
			ampm );
		
		CLStyleRecord timestyle;
		SetUpStyle( kMsgTimeStamp, &timestyle );
		win->twText.StyleInsert( buffer, &timestyle );
		if ( stream )
			{
			// no need to bother with line endings; there aren't any
			fputs( buffer, stream );
			}
		}
	
	// insert the text
	// if there's no text, insert a space anyway, to get the lineheights right
	// else the window fails to scroll correctly
	win->twText.StyleInsert( " ", style );
	if ( text[0] )
		win->twText.StyleInsert( text, style );
	if ( stream )
		{
		// make line endings compatible with C-library conventions if necessary:
		// turn CL's internal \r's into \n's
		SwapLineEndings( const_cast<char *>( text ) );
		
		// write the text to the file
		fputs( text, stream );
		
		// then restore them
		SwapLineEndings( const_cast<char *>( text ) );
		
		fflush( stream );		// is this really necessary?
								// Apparently Algernon relies on it.
		}
	
	// restore the selected text
	win->twText.SelectText( start, stop );
	
	// restore the top line if the selection is not at the end
	if ( topLine == maxLine )
		topLine = win->GetRange();
	else
		{
		topLine -= linesKilled;
		if ( 0 > topLine )
			topLine = 0;
		}
	win->twText.SetTopLine( topLine );
	
	// fix the damn bug already
	if ( atBottom )
		win->twText.ScrollToBottom();
	
	__Verify_noErr( EnableScreenUpdates() );
	
	// reset the scroll bar range
	win->SetRange();
}


/*
**	SaveTextLog()
**
**	save to the text log
*/
static void
SaveTextLog( const char * fname )
{
	// close the file
	std::FILE * stream = gTextFile;
	if ( stream )
		{
		fclose( stream );
		gTextFile = nullptr;
		}
	
	// if a file name was given then open it
	if ( fname )
		{
		DTSFileSpec savedDir;
		savedDir.GetCurDir();
		
		// locate the outer log folder
		DTSFileSpec logFolder;
		logFolder.SetFileName( kTextLogsFolder );
		
		DTSError result = logFolder.SetDir();
		
		// if it didn't exist, make it
		if ( noErr != result )
			{
			result = logFolder.CreateDir();
			if ( noErr == result )
				result = logFolder.SetDir();
			}
			
		if ( noErr == result
		&&	 gLogCharName[0] )
			{
			// locate the player log folder
			logFolder.SetFileName( gLogCharName );
			
			result = logFolder.SetDir();

			// if it didn't exist, make it
			if ( noErr != result )
				{
				result = logFolder.CreateDir();
				if ( noErr == result )
					result = logFolder.SetDir();
				}
			}
			
		if ( noErr == result )
			{
			DTSFileSpec logFile;
			logFile.SetFileName( fname );
			gTextFile = logFile.fopen( "w" );
//			if ( gTextFile )
//				logFile.SetTypeCreator( rTextFREF, 0 );
			}
		savedDir.SetDirNoPath();
		}
}


/*
**	StartTextLog()
**
**	start a new text log
*/
void
StartTextLog()
{
	bool movie = CCLMovie::IsReading();
	
	if ( gPrefsData.pdSaveTextLog
	&&	 not ( movie && gPrefsData.pdNoMovieTextLogs ) )
		{
		DTSDate now;
		now.Get();
		
		// used to be "CL Log yyyy/mm/dd hh.mm.ss"
		// but OSX prefers better extensions
#define TEXT_LOG_NAME_TMPL		"CL Log %.4d/%.2d/%.2d %.2d.%.2d.%.2d.txt"
		
		char buffer[ 32 ];
		snprintf( buffer, sizeof buffer, TEXT_LOG_NAME_TMPL,
			now.dateYear,
			now.dateMonth,
			now.dateDay,
			now.dateHour,
			now.dateMinute,
			now.dateSecond );
		
		// set the log char name before starting the file, so it goes in the right folder
		StringCopySafe( gLogCharName, 
				movie ? kTextLogsMovieSubfolder : gPrefsData.pdCharName,
				sizeof gLogCharName );
		
		SaveTextLog( buffer );
		}
	else
		{
		gLogCharName[ 0 ] = '\0';
		SaveTextLog( nullptr );
		}
}


/*
**	CloseTextLog()
**
**	close the text log file
*/
void
CloseTextLog()
{	
	if ( std::FILE * stream = gTextFile )
		{
		fclose( stream );
		gTextFile = nullptr;
		}
}

#pragma mark -


#if ! TEXTFIELD_USES_MLTE
/*
**	ScrollTextField::Click()
**
**	override a click in the text field to provide autoscrolling, with
**	synchronization of the text window's scroll bar during the scroll
*/
bool
ScrollTextField::Click( const DTSPoint& where, ulong time, uint modifiers )
{
	DTSTextFieldPriv * p = priv.p;
	__Check( p );
	if ( not p )
		return false;
	
	// get the TE handle
	TEHandle hTE = p->textTEHdl;
	__Check( hTE );
	if ( not hTE )
		return false;
	
	// we want auto-scrolling during a click
	SetAutoScroll( true );
	
	// hang onto the system's hook, so we can daisychain to it
	prevClickLoop = (**hTE).clickLoop;
	
	// now install our custom click-loop hook
	::TESetClickLoop( NewTEClickLoopUPP( DoClickLoop ), hTE );
	
	// click!
	bool result = TextFieldType::Click( where, time, modifiers );
	
	// remove our modifications
	::TESetClickLoop( prevClickLoop, hTE );
	prevClickLoop = nullptr;
	
	// turn auto-scroll back off, so that subsequent incoming text doesn't blow
	// away the user's chosen place
	SetAutoScroll( false );
	
	return result;
}


/*
**	ScrollTextField::DoClickLoop()		[static]
**
**	glue routine to install into the TERecord
**	it dispatches the click hook to the non-static member function
*/
DTSBoolean
ScrollTextField::DoClickLoop( TEPtr pTE )
{
	bool result = true;
	
	// grr, TEGetStyleRecord() needs a TEHandle, but we have only a TEPointer...
	// so fish it out directly
	TEStyleHandle styleRec = * reinterpret_cast<TEStyleHandle *>( &pTE->txFont );
	__Check( styleRec );
	if ( styleRec )
		{
		// grab the refcon, which is actually a pointer to a StyledTextField
		ScrollTextField * text =
			reinterpret_cast<ScrollTextField *>( (*styleRec)->teRefCon );
		
		// use that to dispatch to the real click loop function
		// make sure that we don't get our wires crossed!
		// also, don't throw exceptions into the ROM
		if ( text  &&  text->priv.p  &&  *text->priv.p->textTEHdl == pTE )
			{
			try {
				result = text->MyClickLoop( pTE );
				}
			catch ( ... ) {}
			}
		}
	
	return result;
}


/*
**	ScrollTextField::MyClickLoop()
**
**	this gets called repeatedly while mouse is down inside the text field.
**	We take that opportunity to update the scroll bar.
*/
bool
ScrollTextField::MyClickLoop( TEPtr pTE )
{
	bool result = true;
	
	//
	// invoke the system's own click-loop hook, which was installed just prior
	// to processing the click, via the call to SetAutoScroll(true)
	//
	
	if ( prevClickLoop )
		result = InvokeTEClickLoopUPP( pTE, prevClickLoop );
	
	//
	// now that the text has done whatever autoscrolling it's going to do,
	// we need to bring the scroll bar into sync
	//
	if ( result )
		{
		// but we ought to be careful that we're dealing with the right
		// instances of the text window and textfield.
		DTSView * viewParent = GetParent();
		TextWindow * win = static_cast<TextWindow *>( viewParent );
		if ( win  &&  this == &win->twText )
			{
			DTSRegion saveClip;
			win->GetMask( &saveClip );
			
			// temporarily mask out all but the scroll bar
			DTSRect viewBounds;
			win->twBar.GetBounds( &viewBounds );
			win->SetMask( &viewBounds );
			win->SetRange();
			
			win->SetMask( &saveClip );
			}
		}
	return result;
}
#endif  // ! TEXTFIELD_USES_MLTE

#pragma mark -


/*
**	TextWindow::TextWindow()
**	constructor
*/
TextWindow::TextWindow() :
	twActive( false ),
	twSearchWraps( true ),
	twIgnoreCase( true ),
	theOffset( 0 )
{
	searchstring[ 0 ] = '\0';
}


/*
**	TextWindow::InstallEvents()
**
**	prepare to handle CarbonEvents
*/
OSStatus
TextWindow::InstallEvents()
{
	// set up for scroll wheel support & services
	OSStatus result = InitTarget( GetWindowEventTarget( DTS2Mac(this) ) );
	if ( noErr == result )
		{
		const EventTypeSpec evts[] =
			{
				{ kEventClassMouse,   kEventMouseWheelMoved },
				{ kEventClassService, kEventServiceGetTypes },
				{ kEventClassService, kEventServiceCopy },
				{ kEventClassWindow,  kEventWindowDrawContent },
				{ kEventClassWindow,  kEventWindowBoundsChanged },
			};
		
		result = AddEventTypes( GetEventTypeCount( evts ), evts );
		}
	
	return result;
}


/*
**	TextWindow::GetMetrics()
**	get some useful theme measurements
*/
void
TextWindow::GetMetrics()
{
	OSStatus result;
	result = GetThemeMetric( kThemeMetricScrollBarWidth, &sBarWd );
	if ( noErr != result )
		sBarWd = 15;
	
	result = GetThemeMetric( kThemeMetricScrollBarOverlap, &sBarInset );
	if ( noErr != result )
		sBarInset = 0;
	
	result = GetThemeMetric( kThemeMetricResizeControlHeight, &sGrowHt );
	if ( noErr != result )
		sGrowHt = 15;
}


/*
**	TextWindow::DoDraw()
**
**	draw the text window
*/
void
TextWindow::DoDraw()
{
	// erase the text area
	DTSRect box;
	GetBounds( &box );
	box.rectRight += sBarInset - sBarWd;	// not the scrollbar
	Erase( &box );
	
	// call through
	DTSWindow::DoDraw();
}


/*
**	TextWindow::Activate()
**
**	check the menu
**	activate the window
*/
void
TextWindow::Activate()
{
	gDTSMenu->SetFlags( mCopy,       kMenuNormal  );
	gDTSMenu->SetFlags( mSelectAll,  kMenuNormal  );
	gDTSMenu->SetFlags( mFind,       kMenuNormal  );
	gDTSMenu->SetFlags( mFindAgain,  kMenuNormal  );
	gDTSMenu->SetFlags( mTextWindow, kMenuChecked );
	
	DTSWindow::Activate();
	
	twActive = true;
	HiliteSelection();
}


/*
**	TextWindow::Deactivate()
**
**	uncheck the menu
**	deactivate the window
*/
void
TextWindow::Deactivate()
{
	gDTSMenu->SetFlags( mCopy,       kMenuDisabled );
	gDTSMenu->SetFlags( mSelectAll,  kMenuDisabled );
	gDTSMenu->SetFlags( mFind,       kMenuDisabled );
	gDTSMenu->SetFlags( mFindAgain,  kMenuDisabled );
	gDTSMenu->SetFlags( mTextWindow, kMenuNormal   );
	
	twActive = false;
	twText.Deactivate();
	
	DTSWindow::Deactivate();
}


/*
**	TextWindow::Size()
**
**	remember the size of the window
**	move the scroll bar
*/
void
TextWindow::Size( DTSCoord h, DTSCoord v )
{
	// place the scroll bar
//	twBar.Hide();
	twBar.Move( h - sBarWd + sBarInset, -sBarInset );
	twBar.Size( sBarWd, v - sGrowHt + 2 * sBarInset );
//	twBar.Show();
	
	// place the text field
//	twText.Hide();
	twText.Size( h - 2 * kGameInputMargin - sBarWd + sBarInset,
				 v - 2 * kGameInputMargin );
	twText.SelectText( 0x7FFF, 0x7FFF );
//	twText.Show();
	
	// call through
	DTSWindow::Size( h, v );

	// reflow the text
	int numLines = twText.GetNumLines();
	int visLines = twText.GetVisLines();
	int firstLine = numLines - visLines;
	if ( firstLine < 0 )
		firstLine = 0;
	twText.SetTopLine( firstLine );
	
	// set the range on the scroll bar
	SetRange();
}


/*
**	TextWindow::Click()
**
**	let the user select text
**	the text library wants the text field to be active when we click
*/
bool
TextWindow::Click( const DTSPoint& loc, ulong time, uint modifiers )
{
	twText.Activate();
	bool bResult = DTSWindow::Click( loc, time, modifiers );
	HiliteSelection();
	
	return bResult;
}


/*
**	TWTextField::Idle()
**
**	Adjust the cursor for the text field
*/
void
TextWindow::Idle()
{
	twText.Idle();
	
	// determine how much we need to sleep
	if ( not gInBack )
		{
		int sleep = 6;
		if ( not gPlayingGame )
			{
			int start, stop;
			twText.GetSelect( &start, &stop );
			if ( start != stop
			||	 0 == start )
				{
				// could be longer if using mouse-moved events
				// to manage the cursor
				sleep = 60;
				}
			else
				sleep = GetCaretTime();
			}
		gDTSApp->SetSleepTime( sleep );
		}
	
	DTSPoint mouseLoc;
	GetMouseLoc( &mouseLoc );
	WindowRef w = DTS2Mac(this);
	DTSRegion visRgn;
	
	GetPortVisibleRegion( GetWindowPort( w ), DTS2Mac( &visRgn ) );
	
	DTSRect viewBounds;
	twText.GetBounds( &viewBounds );
	if ( mouseLoc.InRegion( &visRgn )
	&&	 mouseLoc.InRect( &viewBounds )
	&&	 not gCursor )
		{
		gCursor = iBeamCursor;
		}
}


/*
**	TextWindow::KeyStroke()
**
**	handle navigation keys locally; else
**	bring the game window to the front and give it this keystroke.
*/
bool
TextWindow::KeyStroke( int ch, uint modifiers )
{
	bool bResult = false;
	bool bRange = false;
	
	if ( not (modifiers & kKeyModMenu) )
		{
		bResult = true;		// we might handle it...
		int newtop;
		switch ( ch )
			{
			case kHomeKey:
				twText.SetTopLine( 0 );
				bRange = true;
				break;
			
			case kEndKey:
				newtop = twText.GetNumLines() - twText.GetVisLines();
				if ( newtop < 0 )
					newtop = 0;
				twText.SetTopLine( newtop );
				SetRange();
				break;
				
			case kPageUpKey:
				newtop = twText.GetTopLine() - twText.GetVisLines() + 1;
				if ( newtop < 0 )
					newtop = 0;
				twText.SetTopLine( newtop );
				bRange = true;
				break;
			
			case kPageDownKey:
				{
				int vislines = twText.GetVisLines();
				newtop = twText.GetTopLine() + vislines - 1;
				if ( newtop > twText.GetNumLines() - vislines  )
					newtop = twText.GetNumLines() - vislines;
				if ( newtop < 0 )
					newtop = 0;
				twText.SetTopLine( newtop );
				bRange = true;
				} break;
			
			default:
				{
				bResult = false;	// nope, we didn't (yet)
				if ( DTSWindow * win = gGameWindow )
					{
					win->GoToFront();
					bResult = win->KeyStroke( ch, modifiers );
					}
				} break;
			}
		}
	
	if ( bRange )
		SetRange();
	
	return bResult;
}


/*
**	TextWindow::DoCmd()
**
**	handle a menu command
*/
bool
TextWindow::DoCmd( long menuCmd )
{
	bool result = true;
	switch ( menuCmd )
		{
		case mCopy:
			twText.Copy();
			break;
		
		case mSelectAll:
			twText.SelectText( 0, 0x7FFF );
			HiliteSelection();
			break;
		
		case mFind:
			DoFind();
			break;
			
		case mFindAgain:
			DoFindAgain();
			break;

		default:
			result = false;
			break;
		}
	
	return result;
}


/*
**	TextWindow::SetRange()
**
**	set the range on the scroll bar
*/
void
TextWindow::SetRange()
{
	// set the maximum
	int barMax = GetRange();
	twBar.SetRange( 0, barMax );
	
	// set the value
	int topLine = twText.GetTopLine();
	twBar.SetValue( topLine );
	
	// set the page value
	int page = twText.GetVisLines() - 1;
	if ( page <= 0 )
		page = 1;
	twBar.SetDeltaPage( page );
	
	// fat thumbs
	int lines = twText.GetVisLines();
	twBar.SetThumbSize( lines );
}


/*
**	TextWindow::GetRange()
**
**	return the number of lines that are not visible
*/
int
TextWindow::GetRange() const
{
	int numLines = twText.GetNumLines();
	int visLines = twText.GetVisLines();
	int barMax = numLines - visLines;
	if ( barMax < 0 )
		barMax = 0;
	
	return barMax;
}


#if 0
/*
**	TextWindow::GetNumLines() -- not needed
**
**	return the number of lines in the field
*/
int
TextWindow::GetNumLines() const
{
	return twText.GetNumLines();
}
#endif // 0


/*
**	TextWindow::HiliteSelection()
**
**	hilite the text selection
*/
void
TextWindow::HiliteSelection()
{
	if ( not twActive )
		return;
	
	// check the selection
	// if there is a selection we want to activate the field
	// if not then we want to deactivate it to hide the blinking cursor
	int start, stop;
	twText.GetSelect( &start, &stop );
	if ( start == stop )
		twText.Deactivate();
	else
		twText.Activate();
}


/*
**	TextWindow::KillOldText()
**
**	remove some lines off the top, to keep us at a safe maximum length
*/
void
TextWindow::KillOldText()
{
	// check the size
	int length = twText.GetTextLength();
	if ( length < kTextMaxLength )
		return;
	
	// save the selection
	int start, stop;
	twText.GetSelect( &start, &stop );
	
	// allocate memory for the text
	char * text = NEW_TAG("TextWindowText") char[ length + 1 ];
	if ( not text )
		return;
	
	// fetch the text
	twText.GetText( text, length + 1 );
	
	// find the minimum number of characters to kill
	int kill = length - kTextMaxLenKill;
	
	// find the first end of line after that
	// and adjust the selection
	const char * str = std::strchr( text + kill, '\r' );
	
	if ( not str )
		{
		start = 0x7FFF;
		stop  = 0x7FFF;
		}
	else
		{
		// move past all endlines
		while ( '\r' == *str )
			++str;
		kill = str - text;
		start -= kill;
		stop  -= kill;
		}
	
	// delete old text
	twText.SelectText( 0, kill );
	twText.Clear();
	
	// dispose of the text
	delete[] text;
	
	// restore the selection if possible
	twText.SelectText( start, stop );
}


/*
**	TextWindow::DoScrollEvent()
**	respond to mouse-wheel events by scrolling the contents of the window
*/
OSStatus
TextWindow::DoScrollEvent( const CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	EventMouseWheelAxis axis;
	SInt32 delta;
	OSStatus err;
	
	//if ( noErr == err )
		{
		err = evt.GetParameter( kEventParamMouseWheelAxis, typeMouseWheelAxis,
					sizeof axis, &axis );
		__Check_noErr( err );
		}
	if ( noErr == err )
		{
		err = evt.GetParameter( kEventParamMouseWheelDelta, typeSInt32,
					sizeof delta, &delta );
		__Check_noErr( err );
		}
	
	if ( noErr == err )
		{
		// only vertical movements do us any actual good
		// but we don't want to refuse horizontal ones
		// otherwise it causes trouble later
		if ( kEventMouseWheelAxisY == axis )
			{
			int oldVal = twBar.GetValue();
			twBar.SetValue( oldVal - delta );
			
			// SetValue() will clamp to current limits. If the value has still
			// changed, even after that, then we should scroll to the new position
			if ( oldVal != twBar.GetValue() )
				twBar.Hit();
			}
		result = noErr;
		}
	
	return result;
}


// enabler flag for exporting traditional MacRoman plain text ('TEXT')
#define CAN_COPY_PLAIN_TEXT			1

// enabler flag for exporting Unicode text (UTF16, probably)
#define CAN_COPY_UNICODE		1

// Use Pasteboard for Services, or Scrap?
#define USE_PASTEBOARD_FOR_SERVICES		1


#if USE_PASTEBOARD_FOR_SERVICES
/*
**	CreateUTIForType()
**
**	given a standardized OSType, return the corresponding UTI, or NULL on error
*/
static CFStringRef
CreateUTIForType( OSType type )
{
	CFStringRef uti( nullptr );
	
	if ( CFStringRef typeStr = CreateTypeStringWithOSType( type ) )
		{
		uti = UTTypeCreatePreferredIdentifierForTag( kUTTagClassOSType, typeStr, nullptr );
		CFRelease( typeStr );
		}
	
	return uti;
}
#endif  // USE_PASTEBOARD_FOR_SERVICES

		
/*
**	TextWindow::DoServiceEvent()
**
**	perform service negotiations
*/
OSStatus
TextWindow::DoServiceEvent( const CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	OSStatus err = noErr;
	
	// cache the text field and its selection bounds
	TextFieldType * field = &twText;
	int start, stop;
	field->GetSelect( &start, &stop );
	
	// CFStrings for the OSTypes of the kinds of data we can copy or paste
	// and maybe the corresponding UTIs.
	// Apparently, the kEventParamService{Paste,Copy}Types arrays are meant to be
	// populated with 4-characters CFStrings (e.g. CFSTR("TEXT")), whereas when you
	// use the Pasteboard API, you need to provide actual UTI strings
	// (CFSTR("com.apple.traditional-mac-plain-text") or similar).
	
	//
	// First, the OSType string and optional UTI for old-style plain text
	//
#if CAN_COPY_PLAIN_TEXT
	static CFStringRef sTypeMacText;
# if USE_PASTEBOARD_FOR_SERVICES
	static CFStringRef sMacTextUTI;
# endif
	
	if ( not sTypeMacText )
		{
		sTypeMacText = CreateTypeStringWithOSType( kScrapFlavorTypeText );	// "TEXT"
		__Check( sTypeMacText );
		
# if USE_PASTEBOARD_FOR_SERVICES
		sMacTextUTI = CreateUTIForType( kScrapFlavorTypeText );
		__Check( sMacTextUTI );
# endif
		}
#endif  // CAN_COPY_PLAIN_TEXT
	
	//
	// next comes the 4-char CFString for Unicode,
	// and maybe its corresponding UTI
	//
#if CAN_COPY_UNICODE
	static CFStringRef sTypeUnicode;
	
# if USE_PASTEBOARD_FOR_SERVICES
	static CFStringRef sUnicodeUTI;
# endif
	
	if ( not sTypeUnicode )
		{
		const OSType kTheType = kScrapFlavorTypeUnicode;			// 'utxt'
		
		sTypeUnicode = CreateTypeStringWithOSType( kTheType );
		__Check( sTypeUnicode );
		
# if USE_PASTEBOARD_FOR_SERVICES
		sUnicodeUTI = CreateUTIForType( kTheType );
		__Check( sUnicodeUTI );
# endif
		}
#endif  // CAN_COPY_UNICODE
	
	//
	// now, decide which kind of service event is it?
	//
	
	switch ( evt.Kind() )
		{
		case kEventServiceGetTypes:
			// this window is effectively read-only, so we can only handle the copy part;
			// we ignore kEventParamServicePasteTypes.
			if ( start < stop )
				{
				CFMutableArrayRef copyTypes;
				err = evt.GetParameter( kEventParamServiceCopyTypes,
							typeCFMutableArrayRef, sizeof copyTypes, &copyTypes );
				__Check_noErr( err );
				if ( noErr == err && copyTypes )
					{
					// if we handle both flavors, put Unicode first,
					// 'cos it's "richer".
#if CAN_COPY_UNICODE
					CFArrayAppendValue( copyTypes, sTypeUnicode );
#endif
#if CAN_COPY_PLAIN_TEXT
					CFArrayAppendValue( copyTypes, sTypeMacText );
#endif
					}
				result = err;
				}
			break;
		
		case kEventServiceCopy:
			{
			// can't copy if there's no selection
			if ( start >= stop )
				break;
			
			int length = field->GetTextLength();
	
#if USE_PASTEBOARD_FOR_SERVICES
			// pasteboardItemIDs are arbitrary so we can just make one up
			const PasteboardItemID kMyPBId = PasteboardItemID( 1 );
			
			// get the destination reference (pasteboard or scrap ref)
			PasteboardRef pb( nullptr );
			err = evt.GetParameter( kEventParamPasteboardRef, typePasteboardRef,
						sizeof pb, &pb );
#else
			ScrapRef scrap( nullptr );
			err = evt.GetParameter( kEventParamScrapRef, typeScrapRef,
						sizeof scrap, &scrap );
#endif
			__Check_noErr( err );
			if ( length && noErr == err )
				{
				// make a local copy of the _entire_ text buffer, because
				// there is no DTSTextField::CopySelectedText()
				
				char * buff = NEW_TAG( "ServiceCopy" ) char[ length + 1 ];
				__Check( buff );
				if ( not buff )
					{
					err = memFullErr;
					break;
					}
				field->GetText( buff, length + 1 );	// null-terminates buffer
				
#if USE_PASTEBOARD_FOR_SERVICES
				err = PasteboardClear( pb );
#else
				err = ClearScrap( &scrap );
#endif
				__Check_noErr( err );
				
#if CAN_COPY_UNICODE
				// make a CFString of just the text selection
				if ( noErr == err )
					{
					if ( CFStringRef cs = CFStringCreateWithBytes( kCFAllocatorDefault,
										(UInt8 *) buff + start, stop - start,
										kCFStringEncodingMacRoman, false ) )
						{
# if USE_PASTEBOARD_FOR_SERVICES
						// put that string onto the pasteboard, as UTF16-with-BOM
						if ( CFDataRef dr = CFStringCreateExternalRepresentation(
												kCFAllocatorDefault,
												cs, kCFStringEncodingUTF16, 0 ) )
							{
							err = PasteboardPutItemFlavor( pb, kMyPBId,
									sUnicodeUTI, dr, kPasteboardFlavorNoFlags );
							__Check_noErr( err );
							CFRelease( dr );
							}
# else
						// ... or onto the scrap, as an array of native-endian UniChars
						CFIndex curLen = CFStringGetLength( cs );
						if ( UniChar * ubuf = NEW_TAG("ServiceCopyUTF") UniChar[ curLen ] )
							{
							CFStringGetCharacters( cs, CFRangeMake(0, curLen), ubuf );
							
							err = PutScrapFlavor( scrap,
										kScrapFlavorTypeUnicode, kScrapFlavorMaskNone,
										curLen * sizeof(UniChar), ubuf );
							__Check_noErr( err );
							delete[] ubuf;
							}
# endif  // USE_PASTEBOARD_FOR_SERVICES
						CFRelease( cs );
						}
					}
#endif  // CAN_COPY_UNICODE
				
				// also as plain text
#if CAN_COPY_PLAIN_TEXT
				if ( noErr == err )
					{
# if USE_PASTEBOARD_FOR_SERVICES
					// make a CFData of just the selected bytes, and put that on pasteboard
					if ( CFDataRef dr = CFDataCreate( kCFAllocatorDefault,
											(const uchar *) buff + start, stop - start ) )
						{
						err = PasteboardPutItemFlavor( pb, kMyPBId,
									sMacTextUTI, dr, kPasteboardFlavorNoFlags );
						CFRelease( dr );
						}
# else
					// ... or put the text directly onto the scrap
					err = PutScrapFlavor( scrap,
							kScrapFlavorTypeText,
//							typeUTF8Text,
							kScrapFlavorMaskNone,
							stop - start, buff + start );
# endif  // USE_PASTEBOARD_FOR_SERVICES
					__Check_noErr( err );
					}
#endif  // CAN_COPY_PLAIN_TEXT
				
				delete[] buff;
				result = err;
				}
			}
			break;
		}
	
	return result;
}


/*
**	TextWindow::DoWindowEvent()
**
**	second-level dispatch, for kEventClassWindow events
*/
OSStatus
TextWindow::DoWindowEvent( const CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	switch ( evt.Kind() )
		{
		case kEventWindowDrawContent:
			DoDraw();
			result = noErr;
			break;
		
		case kEventWindowBoundsChanged:
			{
			UInt32 attr( 0 );
			evt.GetParameter( kEventParamAttributes, typeUInt32, sizeof attr, &attr );
			
			// ignore bounds-changes that are merely moves
			if ( attr & kWindowBoundsChangeSizeChanged )
				{
				HIRect curBounds;
				OSStatus err = evt.GetParameter( kEventParamCurrentBounds,
								typeHIRect, sizeof curBounds, &curBounds );
				__Check_noErr( err );
				if ( noErr == err )
					{
					StDrawEnv saveEnv( this );
					Size( CGRectGetWidth( curBounds ), CGRectGetHeight( curBounds ) );
					DoDraw();
					result = noErr;
					}
				else
					result = err;
				}
			}
			break;
		}
	
	return result;
}


/*
**	TextWindow::HandleEvent()
**
**	dispatch specific carbon events to their handlers
*/
OSStatus
TextWindow::HandleEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	switch ( evt.Class() )
		{
		// handle mouse-wheels
		case kEventClassMouse:
			if ( kEventMouseWheelMoved == evt.Kind() )
				result = DoScrollEvent( evt );
			break;
	
		// handle service events
		case kEventClassService:
			result = DoServiceEvent( evt );
			break;
	
		// and window events
		case kEventClassWindow:
			result = DoWindowEvent( evt );
			break;
		}
	
	return result;
}


/*
**	TextScrollBar::Hit()
**
**	handle a click in the text scroll bar
*/
void
TextScrollBar::Hit()
{
	TextWindow * win = static_cast<TextWindow *>( gTextWindow );
	int curTopLine = win->twText.GetTopLine();
	int newTopLine = GetValue();
	if ( curTopLine != newTopLine )
		win->twText.SetTopLine( newTopLine );
}


#pragma mark -

/*
**	TextWindow::DoFind()
**
**	find a string in the text window
*/
void
TextWindow::DoFind()
{
	if ( not FindDlg::DoFindTextDialog( searchstring, twSearchWraps, twIgnoreCase ) )
		return;
	
	theOffset = 0;
	DoFindAgain();
}


/*
**	TextWindow::DoFindAgain()
**
**	find a string in the text window
*/
void
TextWindow::DoFindAgain()
{
	// allocate memory for the text
	int length = twText.GetTextLength();
	char * text = NEW_TAG("TextWindowFindAgain") char[ length + 1 ];
	if ( not text )
		return;
	
	// fetch the text
	twText.GetText( text, length + 1 );
	
	// search for it from end of previous offset
	int len = std::strlen( searchstring );
	const char * textOffset = twIgnoreCase
		? strcasestr( text + theOffset + len, searchstring )
		: std::strstr( text + theOffset + len, searchstring );
	
	if ( textOffset )
		{
		twText.SetAutoScroll( true );

		theOffset = textOffset - text;
		
		// cheap workaround for TE scrolling issue, when trying to hilight
		// a selection that begins a new line.
		if ( *(textOffset - 1) == 0x0D )
			{
			// first select the desired range NOT including the first character of it
			// that'll cause the window to scroll to the right place
			twText.SelectText( theOffset + 1, theOffset + len - 1 );
			}

		// now select the true range
		twText.SelectText( theOffset, theOffset + len );
		twText.SetAutoScroll( false );

		Activate();
		SetRange();
		}
	else
		{
		Beep();
		if ( twSearchWraps )
			theOffset = 0;
		}
	
	delete[] text;
}



/*
**	FindDlg::Init()
*/
void
FindDlg::Init()
{
	SetText( Item( fdFindTextEdit ), findText );
	SelectText( Item( fdFindTextEdit ), 0, 0x3FFF );
	SetValue( Item( fdSearchWrapsCheck ), fdSearchWraps );
	SetValue( Item( fdIgnoreCaseCheck ), fdIgnoreCase );
}


/*
**	FindDlg::DoFindTextDialog()	[static]
*/
bool
FindDlg::DoFindTextDialog( char * searchstring, bool& searchWraps, bool& ignoreCase )
{
	FindDlg theDialog;
	StringCopySafe( theDialog.findText, searchstring, sizeof theDialog.findText );
	theDialog.fdSearchWraps = searchWraps;
	theDialog.fdIgnoreCase = ignoreCase;
	
	theDialog.Run();
	
	theDialog.fdSearchWraps = theDialog.GetValue( Item( fdSearchWrapsCheck ) );
	theDialog.fdIgnoreCase = theDialog.GetValue( Item( fdIgnoreCaseCheck ) );
	theDialog.GetText( Item( fdFindTextEdit ),
		theDialog.findText, sizeof theDialog.findText );
	
	searchWraps = theDialog.fdSearchWraps;
	ignoreCase = theDialog.fdIgnoreCase;
	
	if ( kHICommandOK == theDialog.Dismissal() )
		{
		StringCopySafe( searchstring, theDialog.findText, kFindTextSize );
		return true;
		}
	
	return false;
}

