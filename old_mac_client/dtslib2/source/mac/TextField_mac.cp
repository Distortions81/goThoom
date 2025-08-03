/*
**	TextField_mac.cp		dtslib2
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**		https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Memory_dts.h"
#include "Platform_dts.h"
#include "TextField_dts.h"
#include "TextField_mac.h"
#include "View_mac.h"

#if TEXTFIELD_USES_MLTE
# pragma GCC diagnostic warning "-Wdeprecated-declarations"
#endif


/*
**	Entry Routines
**
**	DTSTextField
*/

#if TEXTFIELD_USES_MLTE
typedef	TXNObject	TextRef;
#else
typedef	TEHandle	TextRef;
#endif


/*
**	DTSTextFieldPriv
*/
DTSDefineImplementFirmNoCopy(DTSTextFieldPriv)


/*
**	DTSTextFieldPriv::DTSTextFieldPriv()
**
**	constructor
*/
DTSTextFieldPriv::DTSTextFieldPriv() :
	textTEHdl( nullptr ),
#if ! TEXTFIELD_USES_MLTE
	textLastStart( -1 ),
	textLastStop( -1 ),
	textUndoStart( -1 ),
	textUndoStop( -1 ),
	textUndoBuff( nullptr ),
	textAnchor( kTEAnchorNone ),
#endif
	textActive( false )
{
}


/*
**	DTSTextFieldPriv::~DTSTextFieldPriv()
**
**	Dispose of the TE record
*/
DTSTextFieldPriv::~DTSTextFieldPriv()
{
	if ( TextRef hTE = textTEHdl )
		{
#if TEXTFIELD_USES_MLTE
		TXNDeleteObject( hTE );
#else
		TEDispose( hTE );
#endif
		}
	
#if ! TEXTFIELD_USES_MLTE
	delete[] textUndoBuff;
#endif
}


#if TEXTFIELD_USES_MLTE
/*
**	Set1TXNControl()
**	convenience wrapper
*/
static inline OSStatus
Set1TXNControl( TXNObject tx, TXNControlTag tag, UInt32 value )
{
	OSStatus result = TXNSetTXNObjectControls( tx, false,
							1, &tag, (const TXNControlData *) &value );
	__Check_noErr( result );
	return result;
}


/*
**	SetTXNFontSize()
**	set the font and size of a TXNObject
*/
static OSStatus
SetTXNFontSize( TXNObject tx, const char * fname, int fsize )
{
	ATSFontRef theFont = 0;
	Fixed theSize = fsize << 16;
	
	// no fontname means "use application font"
	if ( not fname || not fname[0] )
		{
		// 1-time fetching of the ATSFontRef for the application font
		static ATSFontRef sDefFont;
		static Fixed sDefSize;
		if ( not sDefFont )
			{
			if ( CTFontRef ctf = CTFontCreateUIFontForLanguage( kCTFontApplicationFontType, 0, nullptr ) )
				{
				sDefFont = CTFontGetPlatformFont( ctf, nullptr );
				sDefSize = int( CTFontGetSize( ctf ) ) << 16;	// int-to-Fixed
				CFRelease( ctf );
				}
			}
		
		theFont = sDefFont;
		if ( not fsize )
			theSize = sDefSize;
		}
	else
	if ( CFStringRef cs = CreateCFString( fname ) )
		{
		theFont = ATSFontFindFromName( cs, kNilOptions );
		__Check( theFont );
		CFRelease( cs );
		}
	
	TXNTypeAttributes atts[] =
		{
			{ kATSUFontTag, sizeof theFont, {} },
			{ kATSUSizeTag, sizeof theSize, {} },
			{ kTXNTextEncodingAttribute, kTXNTextEncodingAttributeSize, {} },
			{ kATSUStyleRenderingOptionsTag, sizeof( ATSStyleRenderingOptions ), {} },
		};
	const ItemCount kNumAttrs = sizeof atts / sizeof atts[0];
	
	atts[ 0 ].data.dataValue = theFont;
	atts[ 1 ].data.dataValue = theSize;
	atts[ 2 ].data.dataValue = kTextEncodingMacRoman;
	
	// control anti-aliasing?
	if ( fsize < 11 && fsize != 0 )
		atts[ 4 ].data.dataValue = kATSStyleNoAntiAliasing;
	
	OSStatus result = TXNSetTypeAttributes( tx, kNumAttrs, atts,
						kTXNStartOffset, kTXNEndOffset );
	__Check_noErr( result );
	
	return result;
}
#endif  // TEXTFIELD_USES_MLTE


/*
**	DTSTextField::Init()
**
**	create the TE record
*/
DTSError
DTSTextField::Init( DTSView * parent, const DTSRect * box,
					const char * fontname /* =nullptr */, int fontsize /* =0 */ )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return -1;
	
	TextRef hTE = p->textTEHdl;
	if ( hTE )
		{
#if TEXTFIELD_USES_MLTE
		TXNDeleteObject( hTE );
#else
		TEDispose( hTE );
#endif  // TEXTFIELD_USES_MLTE
		}
	
	Detach();
	Attach( parent );
	
	StDrawEnv saveEnv( parent );
	
#if TEXTFIELD_USES_MLTE
	HIRect r = CGRectMake( box->rectLeft,
					box->rectTop,
					box->rectRight - box->rectLeft,
					box->rectBottom - box->rectTop );
	enum
		{
		kFrameOpts = kTXNDontDrawSelectionWhenInactiveMask
		};
	
	hTE = nullptr;
	OSStatus err = TXNCreateObject( &r, kFrameOpts, &hTE );
	__Check_noErr( err );
	p->textTEHdl = hTE;
	
	if ( noErr == err && hTE )
		{
		// TXNObjects need to be attached to windows, not just views
		// but 'parent' might not be a DTSWindow. So we have to grope upward
		// til we find a window
		for ( DTSView * test = parent; test; test = test->GetParent() )
			{
			if ( DTSWindow * testW = dynamic_cast<DTSWindow *> ( test ) )
				{
				WindowRef win = DTS2Mac( testW );
				err = TXNAttachObjectToWindowRef( hTE, win );
				__Check_noErr( err );
				break;
				}
			}
		// TODO: complain if no window found
		}
	
	if ( noErr == err )
		err = SetTXNFontSize( hTE, fontname, fontsize );
	if ( noErr == err )
		__Verify_noErr( TXNSetScrollbarState( hTE, kScrollBarsSyncWithFocus ) );
	
	if ( err )
		return err;
	
#else  // ! TEXTFIELD_USES_MLTE
	
#ifndef AVOID_QUICKDRAW
	GrafPtr port = DTS2Mac(parent);
	int savefont = GetPortTextFont( port );
	int savesize = GetPortTextSize( port );
#endif  // ! AVOID_QUICKDRAW
	
	if ( fontname )
		{
		parent->SetFont( fontname );
		parent->SetFontSize( fontsize );
		}
	
	hTE = TENew( DTS2Mac(box), DTS2Mac(box) );
	
#ifndef AVOID_QUICKDRAW
	TextFont( savefont );
	TextSize( savesize );
#endif
	
	if ( not hTE )
		return memFullErr;
	
	TEAutoView( true, hTE );
	p->textTEHdl = hTE;
#endif  // TEXTFIELD_USES_MLTE
	
	Move( box->rectLeft, box->rectTop );
	Size( box->rectRight - box->rectLeft, box->rectBottom - box->rectTop );
	
	return noErr;
}


/*
**	DTSTextField::Activate()
**
**	activate the TE record
*/
void
DTSTextField::Activate()
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	TXNFocus( hTE, true );
#else
	StDrawEnv saveEnv( this );
	TEActivate( hTE );
#endif
	
	p->textActive = true;
}


/*
**	DTSTextField::Deactivate()
**
**	deactivate the TE record
*/
void
DTSTextField::Deactivate()
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	TXNFocus( hTE, false );
#else
	StDrawEnv saveEnv( this );
	TEDeactivate( hTE );
#endif
	
	p->textActive = false;
}


/*
**	DTSTextField::SetText()
**
**	set the TE record text
*/
void
DTSTextField::SetText( const char * text )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	__Verify_noErr( TXNSetData( hTE, kTXNTextData,
		text, strlen(text), kTXNStartOffset, kTXNEndOffset ) );
#else
	p->TextToUndo();
	
	StDrawEnv saveEnv( this );
	
	TESetSelect( 0, 0x7FFF, hTE );
	TEDelete( hTE );
	TEInsert( text, std::strlen(text), hTE );
	p->textAnchor = kTEAnchorNone;
	
//	TESetText( text, strlen(text), hTE );
	
	p->textLastStart = -1;
	p->textLastStop  = -1;
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::GetText()
**
**	get the TE record text
*/
void
DTSTextField::GetText( char * text, size_t bufferSize ) const
{
	if ( DTSTextFieldPriv * p = priv.p )
		p->GetText( text, bufferSize );
}


/*
**	DTSTextFieldPriv::GetText()
**
**	get the TE record text
*/
void
DTSTextFieldPriv::GetText( char * text, size_t bufferSize ) const
{
	TextRef hTE = textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	ByteCount length = TXNDataSize( hTE );
	if ( length >= bufferSize )
		length = bufferSize - 1;
	Handle h = nullptr;
	OSStatus err = TXNGetData( hTE, kTXNStartOffset, kTXNEndOffset, &h );
	__Check_noErr( err );
	if ( noErr == err )
		{
		memcpy( text, *h, length );
		text[ length ] = '\0';
		}
	if ( h )
		DisposeHandle( h );
#else  // ! TEXTFIELD_USES_MLTE
	int length = (*hTE)->teLength;
	if ( length >= (int) bufferSize )
		length = bufferSize - 1;
	memcpy( text, *(*hTE)->hText, length );
	text[ length ] = 0;
#endif  // TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::GetTextLength()
**
**	return the number of characters
*/
int
DTSTextField::GetTextLength() const
{
	DTSTextFieldPriv * p = priv.p;
	return p ? p->GetTextLength() : 0;
}


/*
**	DTSTextFieldPriv::GetTextLength()
**
**	return the number of characters
*/
int
DTSTextFieldPriv::GetTextLength() const
{
	TextRef hTE = textTEHdl;
#if TEXTFIELD_USES_MLTE
	return hTE ? TXNDataSize( hTE ) : 0;
#else
	return hTE ? (*hTE)->teLength : 0;
#endif
}


/*
**	DTSTextField::SelectText()
**
**	select text in the TE record text
*/
void
DTSTextField::SelectText( int start, int stop )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
	StDrawEnv saveEnv( this );
	
#if TEXTFIELD_USES_MLTE
	int len = TXNDataSize( hTE );
#else
	int len = (*hTE)->teLength;
#endif
	if ( start < 0 )
		start = 0;
	else
	if ( start > len )
		start = len;
	
	if ( stop < 0 )
		stop = 0;
	else
	if ( stop > len )
		stop = len;
	
	if ( start > stop )
		{
		int temp = start;
		start = stop;
		stop = temp;
		}
#if TEXTFIELD_USES_MLTE
//	if ( 0 == start )
//		0 = kTXNStartOffset;
	if ( len == stop )
		stop = kTXNEndOffset;
	__Verify_noErr( TXNSetSelection( hTE, start, stop ) );
#else
	p->textAnchor = kTEAnchorNone;
	TESetSelect( start, stop, hTE );
#endif
}


/*
**	DTSTextField::GetSelect()
**
**	return the selection
*/
void
DTSTextField::GetSelect( int * start, int * stop ) const
{
	if ( DTSTextFieldPriv * p = priv.p )
		p->GetSelect( start, stop );
}


/*
**	DTSTextFieldPriv::GetSelect()
*/
void
DTSTextFieldPriv::GetSelect( int * start, int * stop ) const
{
	TextRef hTE = textTEHdl;
	if ( not hTE )
		{
		*start = 0;
		*stop  = 0;
		}
	else
		{
#if TEXTFIELD_USES_MLTE
		TXNOffset first( kTXNStartOffset ), last( kTXNEndOffset );
		TXNGetSelection( hTE, &first, &last );
		*start = first;
		*stop = last;
#else
		TEPtr pTE = *hTE;
		*start = pTE->selStart;
		*stop  = pTE->selEnd;
#endif
		}
}


/*
**	DTSTextField::Insert()
**
**	insert the text
*/
void
DTSTextField::Insert( const char * text )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	TXNOffset start, end;
	TXNGetSelection( hTE, &start, &end );
	__Verify_noErr( TXNSetData( hTE, kTXNTextData,
				text, strlen(text), start, end ) );
#else
	p->TextToUndo();
	
	StDrawEnv saveEnv( this );
	
	TEPtr pTE = *hTE;
	if ( pTE->selStart != pTE->selEnd )
		TEDelete( hTE );
	TEInsert( text, std::strlen(text), hTE );
	
	p->textAnchor = kTEAnchorNone;
	GetSelect( &p->textLastStart, &p->textLastStop );
#endif  // ! TEXTFIELD_USES_MLTE
}


#if ! TEXTFIELD_USES_MLTE
/*
**	TEIsStyled()
**	returns whether a TERecord is styled or not
**	irrelevant for MLTE
*/
static inline bool
TEIsStyled( const TERec * pTE )
{
	return pTE && pTE->txSize < 0;
}
#endif  // ! TEXTFIELD_USES_MLTE


/*
**	DTSTextField::Copy()
**
**	copy the TE record selection to the scrap
*/
void
DTSTextField::Copy() const
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	__Verify_noErr( TXNCopy( hTE ) );
#else
	__Verify_noErr( ClearCurrentScrap() );
	
	TECopy( hTE );
	
	// for old-style terecords [only], TECopy() doesn't update the system scrap
	if ( not TEIsStyled( *hTE ) )
		TEToScrap();
#endif
}


/*
**	DTSTextField::Paste()
**
**	Paste the scrap into the TE record
*/
void
DTSTextField::Paste()
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	__Verify_noErr( TXNPaste( hTE ) );
#else
	p->TextToUndo();
	
	StDrawEnv saveEnv( this );
	
	// old-style TERecords don't track the system scrap
	if ( not TEIsStyled( *hTE ) )
		TEFromScrap();
	
	TEPaste( hTE );
	
	p->textAnchor = kTEAnchorNone;
	GetSelect( &p->textLastStart, &p->textLastStop );
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::Clear()
**
**	clear the TE record selection
*/
void
DTSTextField::Clear()
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	__Verify_noErr( TXNClear( hTE ) );
#else
	p->TextToUndo();
	
	StDrawEnv saveEnv( this );
	
	TEDelete( hTE );
	
	p->textAnchor = kTEAnchorNone;
	GetSelect( &p->textLastStart, &p->textLastStop );
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::KeyStroke()
**
**	send a character to the TE record
*/
bool
DTSTextField::KeyStroke( int ch, uint modifiers )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return true;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return true;
	
#if TEXTFIELD_USES_MLTE
	// fake up an event record :-<
	EventRecord er;
	if ( keyDown == gEvent.what )
		er = gEvent;						// assume this is the right event
	else
		{
		er.what = keyDown;
		er.message = ch & charCodeMask;		// *** what about the other bits?
		er.when = TickCount();				// ***
		er.where = (Point){ 0, 0 };			// ***
		er.modifiers = modifiers;			// *** DTS2Mac
		}
	TXNKeyDown( hTE, &er );
#else
# pragma unused( modifiers )
	p->TextToUndo();
	
# ifndef AVOID_QUICKDRAW
	ObscureCursor();
# endif
	
	StDrawEnv saveEnv( this );
	TEKey( ch, hTE );
	p->textAnchor = kTEAnchorNone;
	
	GetSelect( &p->textLastStart, &p->textLastStop );
#endif  // ! TEXTFIELD_USES_MLTE
	
	return true;
}


/*
**	DTSTextField::Idle()
**
**	flash the caret
*/
void
DTSTextField::Idle()
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	TXNIdle( hTE );
#else
	StDrawEnv saveEnv( this );
	TEIdle( hTE );
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::DoDraw()
**
**	redraw the TE record
*/
void
DTSTextField::DoDraw()
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	enum { items = kTXNDrawItemTextAndSelectionMask };	// kTXNDrawItemTextMask??
	__Verify_noErr( TXNDrawObject( hTE, nullptr, items ) );
#else
	Rect box = (*hTE)->viewRect;
# ifndef AVOID_QUICKDRAW
	EraseRect( &box );
# endif
	TEUpdate( &box, hTE );
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::Move()
**
**	move the text field
*/
void
DTSTextField::Move( DTSCoord left, DTSCoord top )
{
	DTSView::Move( left, top );
	
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	HIRect vuRect;
	OSStatus err = TXNGetHIRect( hTE, kTXNViewRectKey, &vuRect );
	__Check_noErr( err );
	if ( noErr == err )
		{
		vuRect.origin = CGPointMake( left, top );
		// TODO: also dest rect?
		TXNSetHIRectBounds( hTE, &vuRect, nullptr, true );
		}
#else
	TEPtr pTE = *hTE;
	
	Rect * rect = &pTE->destRect;
	rect->top  = top;
	rect->left = left;
	rect = &pTE->viewRect;
	rect->top  = top;
	rect->left = left;
	
	StDrawEnv saveEnv( this );
	
	TECalText( hTE );
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::Size()
**
**	size the text field
*/
void
DTSTextField::Size( DTSCoord width, DTSCoord height )
{
	DTSView::Size( width, height );
	
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
# if 1
	TXNResizeFrame( hTE, width, height, 0 );
# else
	HIRect vuRect;
	OSStatus err = TXNGetHIRect( hTE, kTXNViewRectKey, &vuRect );
	__Check_noErr( err );
	if ( noErr == err )
		{
		vuRect.size = CGSizeMake( width, height );
		TXNSetHIRectBounds( hTE, &vuRect, nullptr, true );
		}
# endif  // 1
#else
	// truncate the height
	TEPtr pTE = *hTE;
	int lineHeight = pTE->lineHeight;
	height = (height / lineHeight) * lineHeight;
	
	// change the dest and view rectangles
	Rect * rect = &pTE->destRect;
	rect->bottom = rect->top  + height;
	rect->right  = rect->left + width;
	rect = &pTE->viewRect;
	rect->bottom = rect->top  + height;
	rect->right  = rect->left + width;
	
	// recalibrate the text
	StDrawEnv saveEnv( this );
	
	TECalText( hTE );
#endif  // TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::SetJust()
**
**	set the justification
*/
void
DTSTextField::SetJust( int just )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	switch ( just )
		{
		case kJustRight:
			just = kTXNFlushRight;
			break;
		
		case kJustCenter:
			just = kTXNCenter;
			break;
		
		default:
			just = kTXNFlushDefault;
			break;
		}
	
	Set1TXNControl( hTE, kTXNJustificationTag, just );
#else
	switch ( just )
		{
		case kJustRight:
			just = teFlushRight;
			break;
		
		case kJustCenter:
			just = teCenter;
			break;
		
		default:
			just = teFlushDefault;
			break;
		}
	
	TESetAlignment( just, hTE );
#endif  // TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::Click()
**
**	handle a click in a text field
*/
bool
DTSTextField::Click( const DTSPoint& loc, ulong time, uint modifiers )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return true;
	
	TextRef hTE = p->textTEHdl;
	if ( hTE
	&&   p->textActive )
		{
#if TEXTFIELD_USES_MLTE
		EventRecord er;
		if ( mouseDown == gEvent.what )
			er = gEvent;  // assume this is the right event
		else
			{
			er.what = mouseDown;
			WindowRef w = TXNGetWindowRef( hTE );
			er.message = reinterpret_cast<ulong>( w );
			er.when = time;
			er.where = * DTS2Mac( &loc );	// *** local to global?
			er.modifiers = modifiers;		// *** DTS2Mac
			}
		TXNClick( hTE, &er );
#else  // ! TEXTFIELD_USES_MLTE
# pragma unused( time )
		StDrawEnv saveEnv( this );
		
		Boolean bExtend = false;
		if ( modifiers & kKeyModShift )
			bExtend = true;
			
		TEClick( *DTS2Mac(&loc), bExtend, hTE );
		
		// find where the mouse went _up_
		Point selPoint;
		GetMouse( &selPoint );
		
		// calc distances from that spot to edges of selection
		int offset = TEGetOffset( selPoint, hTE );
		int distToStart = ( offset - (**hTE).selStart );
		int distToEnd = ( offset - (**hTE).selEnd );
		
		// ensure both are positive
		if ( distToStart < 0 )
			distToStart = -distToStart;
		if ( distToEnd < 0 )
			distToEnd = -distToEnd;
		
		// whichever one is nearest the mouse clickLoc is the new free end
		// ergo, set the anchored end to the faraway one
		if ( distToStart < distToEnd )
			{
			// left edge is nearer the mouse
			p->textAnchor = kTEAnchorRight;
			}
		else
			// right edge nearer, or equidistant
			p->textAnchor = kTEAnchorLeft;
#endif  // ! TEXTFIELD_USES_MLTE
		}
	
	return true;
}


/*
**	DTSTextField::GetFormat()
**
**	return the number of lines and an array containing
**	the index of the first character of each line
**	the caller is responsible for disposing of this memory
*/
TFFormat *
DTSTextField::GetFormat() const
{
#if TEXTFIELD_USES_MLTE
	// not supported
	return nullptr;
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return nullptr;
	
	TFFormat * format = nullptr;
	
	if ( TEHandle hTE = p->textTEHdl )
		{
		int numLines = (*hTE)->nLines;
		size_t formatLen = offsetof(TFFormat, tfLineStarts)
						 + sizeof(format->tfLineStarts[0]) * numLines;
		format = reinterpret_cast<TFFormat *>( NEW_TAG("DTSTextFormat") char[ formatLen ] );
		TestPointer( format );
		
		if ( format )
			{
			format->tfNumLines = numLines;
			uint * tfls = format->tfLineStarts;
			const SInt16 * tels = (*hTE)->lineStarts;
			for ( ;  numLines > 0;  --numLines )
				*tfls++ = *tels++;
			}
		}
	
	return format;
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::GetTopLine()
**
**	return the line that is at the top of the view rect
*/
int
DTSTextField::GetTopLine() const
{
#if TEXTFIELD_USES_MLTE
	// not implemented? ***
	return 0;
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return 0;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return 0;
	
	TEPtr pTE = *hTE;
	int topLine = pTE->viewRect.top - pTE->destRect.top;
	topLine /= pTE->lineHeight;
	
	return topLine;
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::SetTopLine()
**
**	scroll the line to the top of the view rect
*/
void
DTSTextField::SetTopLine( int newTopLine )
{
#if TEXTFIELD_USES_MLTE
	// not implemented? ***
# pragma unused( newTopLine )
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
	TEPtr pTE = *hTE;
	int newDestTop = pTE->viewRect.top - newTopLine * pTE->lineHeight;
	int curDestTop = pTE->destRect.top;
	int dv = newDestTop - curDestTop;
	if ( dv != 0 )
		TEPinScroll( 0, dv, hTE );
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::GetVisLines()
**
**	return the number of visible lines
*/
int
DTSTextField::GetVisLines() const
{
#if TEXTFIELD_USES_MLTE
	// not implemented? ***
	// ... could vary from moment to moment, if multiple text styles are in use
	return 0;
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return 0;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return 0;
	
	// truncate the height
	TEPtr pTE = *hTE;
	int numVisLines = pTE->viewRect.bottom - pTE->viewRect.top;
	numVisLines /= pTE->lineHeight;
	
	return numVisLines;
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::SetAutoScroll()
**
**	Turn auto-scroll on or off; returns prior setting
*/
bool
DTSTextField::SetAutoScroll( bool scrollmode )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return false;
	
	TextRef hTE = p->textTEHdl;
	if ( not hTE )
		return false;
	
#if TEXTFIELD_USES_MLTE
	bool bWasScrolling = false;
	const TXNControlTag tag = kTXNAutoScrollBehaviorTag;
	TXNControlData data = { 0 };
	OSStatus err = TXNGetTXNObjectControls( hTE, 1, &tag, &data );
	__Check_noErr( err );
	if ( noErr == err )
		bWasScrolling = (kTXNAutoScrollNever != data.uValue);
	
	data.uValue = scrollmode ? kTXNAutoScrollInsertionIntoView : kTXNAutoScrollNever;
	Set1TXNControl( hTE, tag, data.uValue );
	
	return bWasScrolling;
#else
	return TEFeatureFlag( teFAutoScroll, scrollmode ? teBitSet : teBitClear, hTE );
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	DTSTextField::Hide()
**
**	hide the field
*/
void
DTSTextField::Hide()
{
#if TEXTFIELD_USES_MLTE
	// not implemented? ***
#else
	if ( DTSTextFieldPriv * p = priv.p )
		if ( TEHandle hTE = p->textTEHdl )
			{
			// make the view rect have 0 width
			TEPtr pTE = *hTE;
			pTE->viewRect.right = pTE->viewRect.left;
			}
#endif  // ! TEXTFIELD_USES_MLTE
	
	DTSView::Hide();
}


/*
**	DTSTextField::Show()
**
**	show the field
*/
void
DTSTextField::Show()
{
#if TEXTFIELD_USES_MLTE
	// not implemented? ***
#else
	if ( DTSTextFieldPriv * p = priv.p )
		if ( TEHandle hTE = p->textTEHdl )
			{
			// restore the view rect's width to the width of the dest rect
			TEPtr pTE = *hTE;
			int left  = pTE->viewRect.left;
			int right = left + pTE->destRect.right - pTE->destRect.left;
			pTE->viewRect.right = right;
			pTE->destRect.left  = left;
			pTE->destRect.right = right;
			}
#endif  // ! TEXTFIELD_USES_MLTE
	
	DTSView::Show();
}


/*
**	DTSTextField::Undo()
**
**	swap the current text/selection with the undo text/selection
*/
void
DTSTextField::Undo()
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
#if TEXTFIELD_USES_MLTE
	if ( TextRef hTE = p->textTEHdl )
		TXNUndo( hTE );
	
#else  // ! TEXTFIELD_USES_MLTE	
	// bail if there is nothing to undo
	if ( p->textUndoStart < 0 )
		return;
	
	// set the current text/selection
	// dispose of the old undo text
	const char * undoBuff = p->textUndoBuff;
	if ( undoBuff )
		{
		// clear the last selection so this SetText saves the undo
		// clear the undo buffer otherwise SetText will dispose of it
		// save the undo selection so we can restore
		p->textLastStart  = -1;
		p->textLastStop   = -1;
		p->textUndoBuff   = nullptr;
		int undoStart = p->textUndoStart;
		int undoStop  = p->textUndoStop;
		
		// set the undo text/selection
		SetText( undoBuff );
		SelectText( undoStart, undoStop );
		
		// _now_ we're done with the undo buffer
		delete[] undoBuff;
		}
	else
		SetText( "" );
#endif  // ! TEXTFIELD_USES_MLTE
}


#if ! TEXTFIELD_USES_MLTE
/*
**	DTSTextFieldPriv::TextToUndo()
**
**	conditionally save the current text/selection
*/
void
DTSTextFieldPriv::TextToUndo()
{
	// the caller is about to change the text
	// if the selection has not changed since the last text change
	// then we bail because we already have the undo information
	int newStart, newStop;
	GetSelect( &newStart, &newStop );
	if ( newStart == textLastStart
	&&   newStop  == textLastStop )
		{
		return;
		}
	
	// collect the new undo text
	int length = GetTextLength() + 1;
	char * newBuff = NEW_TAG("TextUndoBuffer") char[ length ];
	if ( not newBuff )
		return;
	
	GetText( newBuff, length );
	
	// dispose of the old undo text
	delete[] textUndoBuff;
	
	// save the new undo text/selection
	textUndoStart = newStart;
	textUndoStop  = newStop;
	textUndoBuff  = newBuff;
}
#endif  // ! TEXTFIELD_USES_MLTE

