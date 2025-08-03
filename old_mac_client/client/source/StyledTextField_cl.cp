/*
**	StyledTextField_cl.cp		Clan Lord
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

#include "StyledTextField_cl.h"

/*
**	Entry Routines
**
**	StyledTextField
*/


/*
**	Internal Variables
*/


/*
**	StyledTextField::Init()
**
**	create the TE record
*/
DTSError
StyledTextField::Init( DTSView * parent, DTSRect * box,
	const char * fontname /* = nullptr */, int fontsize /* = 0 */ )
{
#if TEXTFIELD_USES_MLTE
	return DTSTextField::Init( parent, box, fontname, fontsize );
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return -1;
	
	TEHandle hTE = p->textTEHdl;
	if ( hTE )
		TEDispose( hTE );
	
	Detach();
	Attach( parent );
	
	StDrawEnv saveEnv( parent );
	
	GrafPtr port = DTS2Mac(parent);
	
	int savefont = GetPortTextFont( port );
	int savesize = GetPortTextSize( port );
	
	if ( fontname )
		{
		parent->SetFont( fontname );
		parent->SetFontSize( fontsize );
		}
	
	DTSError result = noErr;
	hTE = TEStyleNew( DTS2Mac(box), DTS2Mac(box) );
	if ( not hTE )
		result = memFullErr;
	
	TextFont( savefont );
	TextSize( savesize );
	
	p->textTEHdl = hTE;
	
	if ( hTE )
		{
		TEAutoView( false, hTE );
		
		// save a back pointer where we can find it again later on:
		// TextWindow's click-loop handler needs this.
		TEStyleHandle styleH = TEGetStyleHandle( hTE );
		__Check( styleH );
		if ( styleH )
			(*styleH)->teRefCon = reinterpret_cast<long>( this );
		}
	
	Move( box->rectLeft, box->rectTop );
	Size( box->rectRight - box->rectLeft, box->rectBottom - box->rectTop );
	
	return result;
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	StyledTextField::GetNumLines()
*/
int
StyledTextField::GetNumLines() const
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return 0;
	
#if TEXTFIELD_USES_MLTE
	return 0;
#else
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return 0;
	
	const TERec * pTE = *hTE;
	int numLines = pTE->nLines;
	
	// long-standing TE bug: add 1 to nLines if last character is CR
	const char * lastPtr = *pTE->hText + pTE->teLength - 1;
	if ( 0x0D == * (const uchar *) lastPtr )
		++numLines;
	
	if ( numLines < 0 )
		numLines = 0;
	
	return numLines;
#endif  // ! TEXTFIELD_USES_MLTE
}


#if TEXTFIELD_USES_MLTE

/*
**	ATSFontForStyle()
**
**	Find the best ATSFontRef for a given QD font
*/
static ATSFontRef
ATSFontForStyle( SInt16 font, uchar style, SInt16 fsize )
{
	static ATSFontRef sLastFontRef;
	static SInt16 sLastFont;
	static uchar sLastStyle;
	
	// duplicate lookup?
	if ( sLastFontRef && sLastFont == font && sLastStyle == style )
		return sLastFontRef;
	
	ATSFontRef r = 0;
	if ( CTFontRef ctf = CTFontCreateWithQuickdrawInstance( nullptr, font, style, fsize ) )
		{
		r = CTFontGetPlatformFont( ctf, nullptr );
		CFRelease( ctf );
		sLastFontRef = r;
		sLastFont = font;
		sLastStyle = style;
		}
	return r;
}


/*
**	TypeAttributesForStyle()
**
**	given a CLStyleRecord, set matching TXNTypeAttributes in 'oAtts'.
**	Returns the number of attributes set, or 0 on error.
**	Assumes oAtts is plenty big enough.
*/
static ItemCount
TypeAttributesForStyle( const CLStyleRecord * s, TXNTypeAttributes * oAtts )
{
	ItemCount nAttr = 0;
	
	// font, size, style, color?
	if ( ATSFontRef fref = ATSFontForStyle( s->font, s->face, s->size ) )
		{
		oAtts->tag = kATSUFontTag;
		oAtts->size = sizeof fref;
		oAtts->data.dataValue = fref;
		++nAttr;
		++oAtts;
		}
	if ( true )
		{
		Fixed fSize = s->size << 16;	// int to Fixed
		oAtts->tag = kATSUSizeTag;
		oAtts->size = sizeof fSize;
		oAtts->data.dataValue = static_cast<UInt32>( fSize );
		++nAttr;
		++oAtts;
		}
	if ( false /* true */ )	// ignore color for now
		{
		static RGBColor rgb;
		rgb = * DTS2Mac( &s->color );
		
		oAtts->tag = kATSUColorTag;
		oAtts->size = sizeof rgb;
		oAtts->data.dataPtr = &rgb;
		++nAttr;
		++oAtts;
		}
	
	return nAttr;
}


/*
**	IsSameStyle()
**
**	are two CLStyleRecords identical?
*/
static bool
IsSameStyle( const CLStyleRecord& s1, const CLStyleRecord& s2 )
{
	return s1.font == s2.font
		&& s1.face == s2.face
		&& s1.size == s2.size
		&& s1.color.rgbRed   == s2.color.rgbRed
		&& s1.color.rgbGreen == s2.color.rgbGreen
		&& s1.color.rgbBlue  == s2.color.rgbBlue;
}

#else  // ! TEXTFIELD_USES_MLTE

/*
**	MakeStyleScrapHandle()
**
**	set up a toolbox StScrpHandle to match a given CLStyleRecord
*/
static StScrpHandle
MakeStyleScrapHandle( const CLStyleRecord * s )
{
	static StScrpHandle gTheStyle;
	static SInt16 sFontID;
	static SInt16 sFontSize;
	
	// toolbox wants a true Mac handle.
	// so allocate it (once)
	if ( not gTheStyle )
		{
		// round up the necessary size a bit, for comfort
		const int kStyleSize = (sizeof(short) + sizeof(ScrpSTElement) + 15) & ~15;
		
		gTheStyle = reinterpret_cast<StScrpHandle>( NewHandle( kStyleSize ) );
		__Check( gTheStyle );
		
		// also cache the application font ID and size
		Str255 fName;
		Style junk;
		if ( noErr == GetThemeFont( kThemeApplicationFont, smRoman, fName, &sFontSize, &junk ) )
			sFontID = FMGetFontFamilyFromName( fName );
		}
	
	if ( gTheStyle )
		{
		// use the true application font
		SInt16 theFont = s->font;
		SInt16 theSize = s->size;
		if ( applFont == theFont && 12 == theSize )
			{
			theFont = sFontID;
			theSize = sFontSize;
			}
		
		// get the font's measurements
		FontInfo fi;
		__Verify_noErr( FetchFontInfo( theFont, theSize, s->face, &fi ) );
		
		// fill out the StScrpHandle
		StScrpPtr p			= * gTheStyle;
		p->scrpNStyles		= 1;
		ScrpSTElement * lm	= p->scrpStyleTab;
		lm->scrpStartChar	= 0;
		lm->scrpHeight		= fi.ascent + fi.descent + fi.leading;
		lm->scrpAscent		= fi.ascent;
		lm->scrpFont		= theFont;
		lm->scrpFace		= s->face;
		lm->scrpSize		= theSize;
		lm->scrpColor		= * DTS2Mac( &s->color );
		}
	return gTheStyle;
}
#endif  // ! TEXTFIELD_USES_MLTE


/*
**	StyledTextField::StyleInsert()
**
**	insert styled text
*/
void
StyledTextField::StyleInsert( const char * text, const CLStyleRecord * style )
{
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
#if TEXTFIELD_USES_MLTE
	TXNObject hTE = p->textTEHdl;
#else
	TEHandle hTE = p->textTEHdl;
#endif
	if ( not hTE )
		return;
	
#if TEXTFIELD_USES_MLTE
	
	TXNTypeAttributes attrs[ 8 ];		// plenty big enough
//	memset( attrs, 0, sizeof attrs );
	
	// cache last-used style
	static CLStyleRecord sLastStyle;
	if ( not IsSameStyle( *style, sLastStyle ) )
		{
		ItemCount nAttr = TypeAttributesForStyle( style, attrs );
		__Verify_noErr( TXNSetTypeAttributes( hTE, nAttr, attrs, kTXNEndOffset, kTXNEndOffset ) );
		sLastStyle = *style;
		}
	
	__Verify_noErr( TXNSetData( hTE, kTXNTextData,
		text, strlen(text), kTXNEndOffset, kTXNEndOffset ) );
#else
	StDrawEnv saveEnv( this );
	
	// if there's a previous selection, delete it
	const TERec * pTE = *hTE;
	if ( pTE->selStart != pTE->selEnd )
		TEDelete( hTE );
	
	StScrpHandle styleScrap = MakeStyleScrapHandle( style );
	TEStyleInsert( text, std::strlen(text), styleScrap, hTE );
#endif
}


/*
**	StyledTextField::GetTopLine()
**
**	return the line that is at the top of the view rect
*/
int
StyledTextField::GetTopLine() const
{
#if TEXTFIELD_USES_MLTE
	return 0;	// not implemented? ***
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return 0;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return 0;
	
	// get baseline of 1st visible line of text
	Point top;
	top.v = (*hTE)->viewRect.top + 1 /* - (*hTE)->destRect.top */;
	top.h = 1;
	
	// turn that into a character offset
	int offset = TEGetOffset( top, hTE );
	
	// find the first line that starts at or after that offset.
	// safe to deref hTE since nothing in this loop moves memory.
	const TERec * pTE = *hTE;
	for ( int i = 0; i <= pTE->nLines; ++i )
		if ( pTE->lineStarts[i] >= offset )
			return i;
	
	return (*hTE)->nLines;
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	StyledTextField::SetTopLine()
**
**	scroll the line to the top of the view rect
*/
void
StyledTextField::SetTopLine( int newTopLine )
{
#if TEXTFIELD_USES_MLTE
	// not implemented? ***
	(void) newTopLine;
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
	// height of lines [0..newTopLine] in pixels
	int wantHeight = 0;
	if ( newTopLine != 0 )
		wantHeight = TEGetHeight( newTopLine, 1, hTE );
	
	const TERec * pTE = *hTE;
	int newDestTop = pTE->viewRect.top - wantHeight;
	int curDestTop = pTE->destRect.top;
	int dv = newDestTop - curDestTop;
	if ( dv != 0 )
		TEPinScroll( 0, dv, hTE );
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	StyledTextField::GetVisLines()
**
**	return the number of visible lines that _could_ be drawn
**	(which is not necessarily the actual number of lines that are visible!
**	-- the two could be different, if the field isn't full of text yet.)
**	!! probably fails if lineheights differ
*/
int
StyledTextField::GetVisLines() const
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
	
	// truncate the height
	int lineHt = TEGetHeight( 1, 1, hTE );
	int numVisLines = ((*hTE)->viewRect.bottom - (*hTE)->viewRect.top) / lineHt;
	
	return numVisLines;
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	StyledTextField::AtBottom()
**	are we scrolled to the bottom?
*/
bool
StyledTextField::AtBottom() const
{
#if TEXTFIELD_USES_MLTE
	// not implemented? ***
	return true;
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return false;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return false;
	
	// entire height of all text
	int totalHeight = TEGetHeight( 9999, 1, hTE );
	
	int htBelow = totalHeight + (*hTE)->destRect.top - (*hTE)->viewRect.bottom;
	
	// ideally this would test against 0, but let's allow ourselves
	// a bit of slack in case the window height isn't an exact multiple of the lineHeight
	// (as might happen with oddball text sizes)
	return htBelow <= 4;
#endif  // ! TEXTFIELD_USES_MLTE
}


/*
**	StyledTextField::ScrollToBottom()
**	ensure that the last line is visible
*/
void
StyledTextField::ScrollToBottom()
{
#if TEXTFIELD_USES_MLTE
	// autoscroll may take care of this?
#else
	DTSTextFieldPriv * p = priv.p;
	if ( not p )
		return;
	
	TEHandle hTE = p->textTEHdl;
	if ( not hTE )
		return;
	
	TEPinScroll( 0, -9999, hTE );
#endif  // TEXTFIELD_USES_MLTE
}

