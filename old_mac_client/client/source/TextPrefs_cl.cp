/*
**	TextPrefs_cl.cp		ClanLord
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
#include "StyledTextField_cl.h"
#include "Speech_cl.h"
#include "Dialog_mach.h"
#include "GameTickler.h"
#include "BrowserList.h"

// are we using the nib-based Style dialog?
// (basically all ready to go, except for the fact that Quartz/CoreText fonts can't
// replicate [hardly] any of QD's algorithmic font variations... which pretty much ruins
// the whole concept of text styles, as currently envisaged.)
// Now required: non-nib dialogs no longer supported.
#define USE_HI_STYLE_DLOG				1


// are we doing the crazy drawing-QD-into-CG hack, which lets us wriggle past
// the abovementioned stumbling block, at the cost of a near-total loss of self-respect?
#define DRAW_QD_STYLES_INTO_CGIMAGE		1


/*
** Entry functions
*/
/*
void	InitTextStyles();
void	GetColorPrefs();
void	SetUpStyle( int msgClass, CLStyleRecord * rec );
*/


/*
** Internal definitions
*/

// name of the bundled property-list file, minus its ".plist" extension
#define TEXT_STYLES_FILE	CFSTR("TextStyles")

// within that plist dictionary, the styles array itself is associated with this key
#define TEXT_STYLES_KEY		CFSTR("Styles")

/*
	Currently the styles array in the plist file contains some 17 entries
	(corresponding to kMsgDefault up to kMsgLastMsg). Each entry is a dictionary
	with the following keys (some optional):
*/
#define TS_Name		CFSTR("Name")		// string: name of style, to show in dialog

#define TS_FontID	CFSTR("FontID")		// integer [an FMFontFamilyRef, NOT a ThemeFontID!]
										// defaults to applFont if key not present in dict

#define TS_FontName	CFSTR("FontName")	// optional; it would have been smarter to use font-names
										// rather than fontIDs; but we didn't realize that
										// back in 1999-2000. If both this key and the fontID one
										// are present for a single style, the name takes
										// precedence.

#define TS_Face		CFSTR("FontFace")	// integer bitmap (0=normal, 1=bold, etc.)
										// in other words, same as a QuickDraw 'Style' byte.
										// defaults to 0 ('normal')

#define TS_Size		CFSTR("FontSize")	// number: font size, in points. Default: 10pt

#define TS_Color	CFSTR("Color")		// array, containing 4 floats: R,G,B,A (in that order)
										// the alpha component is not used.

#define	TS_Speak	CFSTR("Speak")		// optional boolean. Default is 'false'

// If & when we start recording user styles in CFPreferences, rather than in gPrefsData,
// this here is the preference key to access those style.
// It leads to an array[ 0 .. kMsgMaxStyles ] of dictionaries, which in turn each use the
// above ("TS_XXX") keys & values.

#define pref_TextStyles				CFSTR("TextStyles")		// array: pdStyles & pdSpeaking


// forward declaration
class ColorChoiceDlg;


/*
**	class TextStylesList
**	databrowser that lists the style names
*/
class TextStylesList : public BrowserList
{
	static const DataBrowserPropertyID prop_Name = 'Name';
	
	virtual OSStatus GetItemData(
						DataBrowserItemID		item,
						DataBrowserPropertyID	property,
						DataBrowserItemDataRef	itemData );
	
	virtual void	DoSelectionSetChanged(
						DataBrowserItemID		item,
						DataBrowserItemDataRef	itemData );
	
	ColorChoiceDlg *	mParent;
	
public:
					TextStylesList() :
						mParent( nullptr )
						{}
					~TextStylesList() {}
	
	OSStatus		Init( ColorChoiceDlg * parent, HIViewRef vu );
	OSStatus		PopulateList();
};


/*
**	class ColorChoiceDlg
**
**	present the UI for modifying the text styles used in the game-window sidebar and
**	in the text log window
*/
class ColorChoiceDlg : public HIDialog, public GameTickler
{
	static const OSType Signature = 'Styl';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef		GetNibFileName() const { return CFSTR("Choose Styles"); }
	virtual CFStringRef		GetPositionPrefKey() const { return CFSTR("TextStylesDlgPosition"); }
	
	TextStylesList	ccdStyleList;					// scrolling list
	CLStyleRecord *	curStyles;						// styles to show/change
	int				curStyleIdx;					// selected list entry
	int				curBubbleBlitter;				// opacity choice for bubbles
	int				curFriendBubbleBlitter;			// ditto, friendly bubbles
	bool			curSpeaking[ kMsgLastMsg ];		// is this msg class spoken?
	
public:
					ColorChoiceDlg() :
						curStyles( nullptr ),
						curStyleIdx( -1 )	// != 0, to force 1st-time selection of item #1
						{}
					
					~ColorChoiceDlg()	{ delete[] curStyles; }
	
protected:
// interface
	virtual void	Init();
	virtual void	Exit();
	virtual bool	DoCommand( const HICommandExtended& );
	void			DrawItem( HIViewRef item, CGContextRef ctxt ) const;
	void			DrawSample( CGContextRef ctxt, const CGRect& box ) const;
	void			DrawSwatch( CGContextRef ctxt, const CGRect& box ) const;
#if DRAW_QD_STYLES_INTO_CGIMAGE
	CGImageRef		CreateCGImageForSampleText( const CGSize& dimens ) const;
#endif

	
// implementation
private:
	void	GetStyleName( int, char *, size_t ) const;
	void	SetItems();
	void	SaveInfo();
	void	SetCurrentStyle( int which );
	static OSStatus	EventProc( EventHandlerCallRef call, EventRef evt, void * ud );
	
	// dialog item numbers
	enum
		{
		iCCDStyleList		= 1,
		iCCDColorSwatch,
		iCCDBoldCheck,
		iCCDItalicCheck,
		iCCDUnderlineCheck,
		iCCDOutlineCheck,
		iCCDShadowCheck,
		iCCDCondensedCheck,
		iCCDExtendedCheck,
		iCCDSpeakCheck,
		iCCDOpacityMenu,
		iCCDSampleText,
		};
	enum
		{
		cmd_PickColor		= 'Colr',	// color-picker swatch
		cmd_PickStyle		= 'PkSt',	// click on browser list
		cmd_PickAttribute	= 'PAtr',	// click on any checkbox
		cmd_ToggleSpeak		= '~Spk',	// "speak" checkbox
		cmd_Opacity			= 'Opac',	// opacity popup menu
		};
	
	// this remembers your list choice from one invocation of the dialog to the next
	static int	sLastStyleIndex;
	
	//	menu item numbers
	enum
		{
		mOpacityNormal = 1,
		mOpacityDisabled,
		mOpacity25,		// labeled as "75%" transparent
		mOpacity50,
		mOpacity75		// "25%"
		};
	
	// map opacity menu items to and from blitter IDs
	static int		OpacityToItem( int opacity );
	static int		ItemToOpacity( int item );
	
	friend class TextStylesList;
};


/*
**	Internal data
*/
int ColorChoiceDlg::sLastStyleIndex;

static CFArrayRef sStyleArray;	// array[ 0..kMsgLastMsg ] of text styles
static CFArrayRef sStyleNames;	// array[ 0..kMsgLastMsg ] of style names (as CFStrings)


/*
**	GetColorPrefs()
**
**	show the dialog, get the prefs
*/
void
GetColorPrefs()
{
	ColorChoiceDlg ccd;
	ccd.Run();
}


#pragma mark -

/*
**	TextStylesList::Init()
**
**	prepare the style browser list and fill it up
*/
OSStatus
TextStylesList::Init( ColorChoiceDlg * parent, HIViewRef vu )
{
	mParent = parent;
	
	OSStatus err = InitFromNib( vu );
	if ( noErr == err )
		{
		enum { atts = kDBAttrAlternateColors | kDBAttrAutoHideBars };
		ChangeAttributes( atts, 0 );
		}
	
	return err;
}


/*
**	TextStylesList::PopulateList()
**
**	add all the styles to the DB
*/
OSStatus
TextStylesList::PopulateList()
{
	int nItems = CFArrayGetCount( sStyleNames );
	OSStatus err = AddDBItems( kDataBrowserNoItem, nItems, nullptr, prop_Name );
	return err;
}


/*
**	TextStylesList::GetItemData()
**
**	callback that provides the contents of DB items
*/
OSStatus
TextStylesList::GetItemData(
	DataBrowserItemID		item,
	DataBrowserPropertyID	property,
	DataBrowserItemDataRef	itemData )
{
	// pessimism
	OSStatus result = errDataBrowserPropertyNotSupported;
	
	// do we even know what the heck they're asking for?
	if ( prop_Name == property )
		{
		int ii = item - 1;
		CFStringRef styleName = static_cast<CFStringRef>(
						CFArrayGetValueAtIndex( sStyleNames, ii ) );
		
		result = SetDataBrowserItemDataText( itemData, styleName );
		__Check_noErr( result );
		}
	
	return result;
}


/*
**	TextStylesList::DoSelectionSetChanged()
**
**	callback which indicates that the DB's selection may have changed.
**	We prod the parent dialog into refreshing its UI.
*/
void
TextStylesList::DoSelectionSetChanged(
	DataBrowserItemID /* item */,
	DataBrowserItemDataRef /* itemData */ )
{
	DataBrowserItemID iid = kDataBrowserNoItem;
	GetDBSelectedItem( &iid );
	
	int styleIdx = iid - 1;
	if ( styleIdx != mParent->curStyleIdx )
		{
		mParent->SetCurrentStyle( styleIdx );
		}
}


/*
**	ColorChoiceDlg::ItemToOpacity()		[static]
**
**	map a menu item to a blitter code
*/
int
ColorChoiceDlg::ItemToOpacity( int item )
{
	int result = kBlitterTransparent;
	switch ( item )
		{
		case mOpacity25: result = kBlitter25; break;
		case mOpacity50: result = kBlitter50; break;
		case mOpacity75: result = kBlitter75; break;
		}
	
	return result;
}


/*
**	ColorChoiceDlg::OpacityToItem()		[static]
**
**	map a blitter code to a menu item
**	(inverse of ColorChoiceDlg::ItemToOpacity)
*/
int
ColorChoiceDlg::OpacityToItem( int opacity )
{
	int result = mOpacityNormal;
	switch ( opacity )
		{
		case kBlitter25: result = mOpacity25; break;
		case kBlitter50: result = mOpacity50; break;
		case kBlitter75: result = mOpacity75; break;
		}
	
	return result;
}


/*
**	ColorChoiceDlg::Init()
**
**	prepare to display the dialog
*/
void
ColorChoiceDlg::Init()
{
	// allocate a local copy of the style records
	curStyles = NEW_TAG("CLStyleRecord") CLStyleRecord[ kMsgLastMsg ];
	CheckPointer( curStyles );
	
#if ! USE_SPEECH
	// if speech is turned off, don't even show the checkbox
	SetVisible( Item( iCCDSpeakCheck ), false );
#endif  // ! USE_SPEECH
	
	// create the list box
	HIViewRef listVu = GetItem( Item( iCCDStyleList ) );
	ccdStyleList.Init( this, listVu );
	
	// populate the list, and the styles array
	for ( int ii = kMsgDefault; ii < kMsgLastMsg; ++ii )
		{
		// initialize curStyles and curSpeaking as we go
		SetUpStyle( static_cast<MsgClasses>( ii ), &curStyles[ ii ] );
		curSpeaking[ ii ] = gPrefsData.pdSpeaking[ ii ];
		}
	
	// remember the bubble blitters currently in use
	curBubbleBlitter = gPrefsData.pdBubbleBlitter;
	curFriendBubbleBlitter = gPrefsData.pdFriendBubbleBlitter;
	
	// tell the databrowser how many rows it has
	ccdStyleList.PopulateList();
	
	// pre-scroll the list to the most-recently-used style
	SetCurrentStyle( sLastStyleIndex );
	
	// set up drawing for the color swatch and sample text views
	const EventTypeSpec evt = { kEventClassControl, kEventControlDraw };
	if ( HIViewRef swatch = GetItem( Item( iCCDColorSwatch ) ) )
		{
		__Verify_noErr( InstallControlEventHandler( swatch, EventProc,
							1, &evt, this, nullptr ) );
		}
	if ( HIViewRef sample = GetItem( Item( iCCDSampleText ) ) )
		{
		__Verify_noErr( InstallControlEventHandler( sample, EventProc,
							1, &evt, this, nullptr ) );
		}
	
	ccdStyleList.RevealDBSelection();
}


/*
**	ColorChoiceDlg::Exit()
**
**	the dialog is going away
*/
void
ColorChoiceDlg::Exit()
{
	if ( kHICommandOK == Dismissal() )
		SaveInfo();
	
	// remember last-used style
	sLastStyleIndex = curStyleIdx;
	
	// dispose of the list
	ccdStyleList.Exit();
}


/*
**	ColorChoiceDialog::SetCurrentStyle()
**
**	inform the dialog that a new style category was chosen
*/
void
ColorChoiceDlg::SetCurrentStyle( int which )
{
	// safety check
	if ( which < kMsgDefault || which >= kMsgLastMsg )
		which = kMsgDefault;
	
	// select item in list
	if ( curStyleIdx != which )
		{
		curStyleIdx = which;
		
		DataBrowserItemID iid = which + 1;
		ccdStyleList.SetDBSelectedItem( iid );
		}
	
	// update dependent controls
	SetItems();
}


# if DRAW_QD_STYLES_INTO_CGIMAGE
	// GCC 4.2 doesn't know these
// #  pragma GCC diagnostic push
// #  pragma GCC diagnostic ignored "-Wdeprecated-declarations"

/*
**	CreateCGImageForSampleText()
**
**	use [horribly deprecated] QuickDraw APIs to draw the styled text samples
**	into a CGImageRef.  Sometimes I hate myself.
*/
CGImageRef
ColorChoiceDlg::CreateCGImageForSampleText( const CGSize& sz ) const
{
	int width = rint( sz.width );
	int height = rint( sz.height );
	
	CGContextRef c = nullptr;
	uchar * pData = nullptr;
	
	// create the CGBitmapContext: 32-bit xRGB
	if ( CGColorSpaceRef cspace = CGColorSpaceCreateWithName( kCGColorSpaceGenericRGB ) )
		{
		uint rowbytes = ((width + 15) & ~15) * sizeof(UInt32);	// 16-pixel aligned
		
		enum { kFlags = kCGImageAlphaNoneSkipFirst | kCGBitmapByteOrder32Big };
		
#if MAC_OS_X_VERSION_MIN_REQUIRED <= 1050
		// until 10.6, CGBitmapContextCreate() did not know how to allocate & manage
		// its own data, so we may have to provide a buffer for it to use.
		if ( gOSFeatures.osVersionMajor == 10 && gOSFeatures.osVersionMinor < 6 )
			pData = NEW_TAG( __PRETTY_FUNCTION__ ) uchar[ height * rowbytes ];
#endif  // 10.5+
		
		c = CGBitmapContextCreate( pData,
					width, height,
					8,	// bits/component
					rowbytes,
					cspace, kFlags );
		__Check( c );
		CFRelease( cspace );
		}
	
	if ( not c )
		{
		delete[] pData;
		return nullptr;
		}
	
	// create a GWorld (which shares the CGBitmapContext's bits) to draw into
	const Rect box = { 0, 0, height, width };		// tlbr
	GWorldPtr pWorld( nullptr );
	OSStatus err = NewGWorldFromPtr( &pWorld,
					k32ARGBPixelFormat,
					&box,
					nullptr, nullptr,				// default color table & GDevice
					0 /* kNativeEndianPixMap */,	// ???
					static_cast<char *>( CGBitmapContextGetData( c ) ),		// data buffer
					CGBitmapContextGetBytesPerRow( c ) );					// row bytes
	__Check_noErr( err );
	
	if ( noErr == err )
		{
		StDrawEnv saveEnv( pWorld );
		
		const char * const samples[] =
			{
			TXTCL_SAMPLETEXT_PLAIN,
			TXTCL_SAMPLETEXT_STRANGER_CLANNING,
			TXTCL_SAMPLETEXT_FRIEND_CLANNING,
			TXTCL_SAMPLETEXT_STRANGER_NO_CLANNING,
			TXTCL_SAMPLETEXT_FRIEND_NO_CLANNING,
			TXTCL_SAMPLETEXT_STRANGER_SHARE,
			TXTCL_SAMPLETEXT_FRIEND_SHARE,
			TXTCL_SAMPLETEXT_ANNOUNCE,
			TXTCL_SAMPLETEXT_STRANGER_FALLEN,
			TXTCL_SAMPLETEXT_FRIEND_FALLEN,
			TXTCL_SAMPLETEXT_STRANGER_TALK,
			TXTCL_SAMPLETEXT_FRIEND_TALK,
			TXTCL_SAMPLETEXT_MYSELF_TALK,
			TXTCL_SAMPLETEXT_BUBBLES,
			TXTCL_SAMPLETEXT_FRIEND_BUBBLES,
			TXTCL_SAMPLETEXT_THINKTO,
			TXTCL_SAMPLETEXT_TIMESTAMP,
			""
			};
		
		// draw the representative text in the sample box
		::BackColor( whiteColor );
		::EraseRect( &box );
		::ForeColor( blackColor );
		::FrameRect( &box );
		
		Rect inner = box;
		::InsetRect( &inner, 3, 3 );
		
		const char * sample = samples[ curStyleIdx ];
		const CLStyleRecord& style = curStyles[ curStyleIdx ];
		
		::TextFont( style.font );
		::TextSize( style.size );
		::RGBForeColor( DTS2Mac( &style.color ) );
		::TextFace( style.face );
		
		::TETextBox( sample, strlen( sample ), &inner, teJustLeft );
		}
	if ( pWorld )
		DisposeGWorld( pWorld );
	
	CGImageRef result = CGBitmapContextCreateImage( c );
	__Check( result );
	CFRelease( c );
	delete[] pData;
	
	return result;
}
// #  pragma GCC diagnostic pop
# endif  // DRAW_QD_STYLES_INTO_CGIMAGE


/*
**	ColorChoiceDlg::DrawSample()
**
**	draw the sample text for style 'curStyleIdx'
*/
void
ColorChoiceDlg::DrawSample( CGContextRef ctxt, const CGRect& inBox ) const
{
#if DRAW_QD_STYLES_INTO_CGIMAGE
	
	if ( CGImageRef img = CreateCGImageForSampleText( inBox.size ) )
		{
		__Verify_noErr( HIViewDrawCGImage( ctxt, &inBox, img ) );
		CFRelease( img );
		}
	
#else  // ! DRAW_QD_STYLES_INTO_CGIMAGE
	
	static const CFStringRef TextSamples[] =
		{
		CFSTR( TXTCL_SAMPLETEXT_PLAIN )	,
		CFSTR( TXTCL_SAMPLETEXT_STRANGER_CLANNING ),
		CFSTR( TXTCL_SAMPLETEXT_FRIEND_CLANNING ),
		CFSTR( TXTCL_SAMPLETEXT_STRANGER_NO_CLANNING ),
		CFSTR( TXTCL_SAMPLETEXT_FRIEND_NO_CLANNING ),
		CFSTR( TXTCL_SAMPLETEXT_STRANGER_SHARE ),
		CFSTR( TXTCL_SAMPLETEXT_FRIEND_SHARE ),
		CFSTR( TXTCL_SAMPLETEXT_ANNOUNCE ),
		CFSTR( TXTCL_SAMPLETEXT_STRANGER_FALLEN ),
		CFSTR( TXTCL_SAMPLETEXT_FRIEND_FALLEN ),
		CFSTR( TXTCL_SAMPLETEXT_STRANGER_TALK ),
		CFSTR( TXTCL_SAMPLETEXT_FRIEND_TALK ),
		CFSTR( TXTCL_SAMPLETEXT_MYSELF_TALK ),
		CFSTR( TXTCL_SAMPLETEXT_BUBBLES ),
		CFSTR( TXTCL_SAMPLETEXT_FRIEND_BUBBLES ),
		CFSTR( TXTCL_SAMPLETEXT_THINKTO ),
		CFSTR( TXTCL_SAMPLETEXT_TIMESTAMP )
		};
	
	// erase background
	CGContextSetGrayFillColor( ctxt, 1.0F, 1.0F );		// white BG
	CGContextFillRect( ctxt, inBox );
	CGContextSetGrayStrokeColor( ctxt, 0.0F, 1.0F );		// black border
	CGRect box = CGRectInset( inBox, 1, 1 );
	CGContextStrokeRectWithWidth( ctxt, box, 1.0F );
	
	CGRect inner = CGRectInset( box, 3, 3 );
	
	const CLStyleRecord& style = curStyles[ curStyleIdx ];
	{
	const DTSColor& c = style.color;
	CGContextSetRGBFillColor( ctxt,
		c.rgbRed / 65536.0F, c.rgbGreen / 65536.0F, c.rgbBlue / 65536.0F, 1 );
	}
	
	// alas we can't use arbitrary fonts & sizes here
	HIThemeTextInfo ti =
		{
		kHIThemeTextInfoVersionZero,
		kThemeStateActive,
		kThemeApplicationFont,
		kHIThemeTextHorizontalFlushLeft,
		kHIThemeTextVerticalFlushTop,
		kHIThemeTextBoxOptionNone,
		kHIThemeTextTruncationNone,
		0,
		false,
# if MAC_OS_X_VERSION_MAX_ALLOWED > 1040
		// SDK 10.5 adds new filler & CTFont fields
		0, nullptr,
# endif
		};
	
# if MAC_OS_X_VERSION_MAX_ALLOWED > 1040
	CTFontRef font = nullptr;
	if ( IsCFMResolved( CTFontCreateWithQuickdrawInstance ) )
		{
		// Many OS X fonts don't have (or can't produce) variants like bold,
		// italic, condensed, etc. In particular, the default application font (Lucida Grande)
		// has only regular and bold faces; and the Classic application font (Geneva) has only
		// regular. Hence the use of Helvetica here, without regard for the actual font
		// specified in the style record.
		CGFloat fontSz = style.size;
		if ( fontSz < 12 )
			fontSz = 12;
		
		font = CTFontCreateWithQuickdrawInstance(
//					nullptr,	// use fontID#, not name
					"\pHelvetica",
					0, // style.font,		// fontID
					style.face & 0x0FFU,	// 8-bit face/style
					fontSz );
		__Check( font );
		
		if ( font )
			{
			ti.version = kHIThemeTextInfoVersionOne;
			ti.fontID = kThemeSpecifiedFont;
			ti.font = font;
			}
		}
# endif  // 10.5+
	
	if ( CFStringRef sample = TextSamples[ curStyleIdx ] )
		{
		__Verify_noErr( HIThemeDrawTextBox( sample,
						&inner, &ti, ctxt, kHIThemeOrientationNormal ) );
		CFRelease( sample );
		}
	
# if MAC_OS_X_VERSION_MAX_ALLOWED > 1040
	if ( font )
		CFRelease( font );
# endif  // 10.5+
	
#endif  // DRAW_QD_STYLES_INTO_CGIMAGE
}


/*
**	ColorChoiceDlg::DrawSwatch()
**
**	draw the interior of the color-swatch image well
*/
void
ColorChoiceDlg::DrawSwatch( CGContextRef ctxt, const CGRect& box ) const
{
	const SInt32 margin = 4;	// the theme metric seems too small to me
//	GetThemeMetric( kThemeMetricImageWellThickness, &margin );
	
	CGRect r = CGRectInset( box, margin, margin );
	
	// fill swatch with the actual color
	const DTSColor& c = curStyles[ curStyleIdx ].color;
	CGContextSetRGBFillColor( ctxt,
			c.rgbRed / 65536.0F, c.rgbGreen / 65536.0F,
			c.rgbBlue / 65536.0F, 1 );
#if 1
	// rounded-rect
	CGFloat x1 = CGRectGetMinX( r ), x2 = CGRectGetMaxX( r );
	CGFloat y1 = CGRectGetMinY( r ), y2 = CGRectGetMaxY( r );
	
	CGContextMoveToPoint( ctxt, x1 + margin, y1 );
	CGContextAddArcToPoint( ctxt, x2, y1, x2, y2, margin );
	CGContextAddArcToPoint( ctxt, x2, y2, x1, y2, margin );
	CGContextAddArcToPoint( ctxt, x1, y2, x1, y1, margin );
	CGContextAddArcToPoint( ctxt, x1, y1, x1 + margin, y1, margin );
	CGContextClosePath( ctxt );
	CGContextFillPath( ctxt );
#else
	CGContextFillRect( ctxt, r );
#endif  // 1
}


/*
**	ColorChoiceDlg::DrawItem()
**
**	repaint the sample-text preview pane, or the color-swatch one
*/
void
ColorChoiceDlg::DrawItem( HIViewRef item, CGContextRef ctxt ) const
{
	CGRect box;
	__Verify_noErr( HIViewGetBounds( item, &box ) );
	
	// which one is it?
	if ( item == GetItem( Item(iCCDSampleText) ) )
		DrawSample( ctxt, box );
	else
	if ( item == GetItem( Item(iCCDColorSwatch) ) )
		DrawSwatch( ctxt, box );
}


/*
**	ColorChoiceDlg::EventProc()
**
**	carbon events for the color-swatch and text-preview HIViews
*/
OSStatus
ColorChoiceDlg::EventProc( EventHandlerCallRef call, EventRef event, void * ud )
{
	CarbonEvent evt( event, call );
	OSStatus result = eventNotHandledErr;
	OSStatus err = noErr;
	ColorChoiceDlg * dlg = static_cast<ColorChoiceDlg *>( ud );
	
	if ( kEventClassControl == evt.Class() )
		{
		// handle redraw
		if ( kEventControlDraw == evt.Kind() )
			{
			CGContextRef ctxt( nullptr );
			HIViewRef whichVu( nullptr );
			
			err = evt.GetParameter( kEventParamCGContextRef, typeCGContextRef,
							sizeof ctxt, &ctxt );
			__Check_noErr( err );
			if ( noErr == err )
				{
				err = evt.GetParameter( kEventParamDirectObject, typeControlRef,
								sizeof whichVu, &whichVu );
				__Check_noErr( err );
				}
			
			// redispatch the draw request
			if ( noErr == err )
				{
				// let the color swatch draw its own (image well) exterior...
				if ( dlg->GetItem( Item(iCCDColorSwatch) ) == whichVu )
					evt.CallNextHandler();
				
				// ... then we draw its interior (or the entirety of the text preview panel)
				dlg->DrawItem( whichVu, ctxt );
				result = noErr;
				}
			}
		}
	
	return result;
}


/*
**	ColorChoiceDlg::DoCommand()
**
**	react to command events
*/
bool
ColorChoiceDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = true;
	
	switch ( cmd.commandID )
		{
		case cmd_PickColor:
			{
			// prompt for a new color
			char prompt[ 80 ];
			char name[ 256 ];
			GetStyleName( curStyleIdx, name, sizeof name );
			
			// run the color-picker dialog
			snprintf( prompt, sizeof prompt, _(TXTCL_CHOOSE_COLOR), name );
			curStyles[ curStyleIdx ].color.ChooseDlg( prompt );
			
			// redraw using new color
			__Verify_noErr( HIViewSetNeedsDisplay( GetItem( Item( iCCDColorSwatch ) ), true ) );
			__Verify_noErr( HIViewSetNeedsDisplay( GetItem( Item( iCCDSampleText ) ), true ) );
			}
			break;
		
		case cmd_PickAttribute:
			// user touched one of the style checkboxes, so update UI (and 'curStyles') to match
			if ( cmd.attributes & kHICommandFromControl )
				{
				HIViewRef src = cmd.source.control;
				HIViewID srcID;
				__Verify_noErr( HIViewGetID( src, &srcID ) );
				
				uint16_t styleBit = 1U << ( srcID.id - iCCDBoldCheck );
				if ( GetValue( srcID ) )
					curStyles[ curStyleIdx ].face |= styleBit;
				else
					curStyles[ curStyleIdx ].face &= ~styleBit;
				
				SetItems();
				}
			break;
		
		case cmd_ToggleSpeak:
			// toggled the "speak" checkbox
			curSpeaking[ curStyleIdx ] = GetValue( Item( iCCDSpeakCheck ) );
			break;
		
		case cmd_Opacity:
			// chose something from the opacity popup menu
			{
			HIViewID opMenu = Item( iCCDOpacityMenu );
			int newValue = GetValue( opMenu );
			
			if ( kMsgBubble == curStyleIdx )
				curBubbleBlitter = ItemToOpacity( newValue );
			else
			if ( kMsgFriendBubble == curStyleIdx )
				curFriendBubbleBlitter = ItemToOpacity( newValue );
			}
			break;
		
		default:
			result = false;
		}
	
	return result;
}


/*
**	ColorChoiceDlg::GetStyleName()
**
**	return the C name of the given style by looking at
**	its resource name if there is one.
**	But if the styles come from a bundled property-list file, then
**	we can just read the name directly from the sStyleNames array.
*/
void
ColorChoiceDlg::GetStyleName( int n, char * name, size_t nameSz ) const
{
	bool got = false;
	if ( sStyleNames )
		{
		__Check( n >= 0 && n < CFArrayGetCount( sStyleNames ) );
		
		if ( CFStringRef s = static_cast<CFStringRef>( CFArrayGetValueAtIndex( sStyleNames, n ) ) )
			{
			if ( CFStringGetCString( s, name, nameSz, kCFStringEncodingMacRoman )
			||   CFStringGetCString( s, name, nameSz, kCFStringEncodingUTF8 ) )
				{
				got = true;
				}
			}
		}
	if ( not got )	// total paranoia
		snprintf( name, nameSz, _(TXTCL_UNNAMED_STYLE), n );
}


/*
**	ColorChoiceDlg::SetItems()
**
**	set dialog items according to a style
*/
void
ColorChoiceDlg::SetItems()
{
	// first do the 8 textface checkboxes
	for ( int i = 0; i < 8; ++i )
		{
		// only non-bubbles can have alternate textfaces
		if ( curStyleIdx != kMsgBubble && curStyleIdx != kMsgFriendBubble )
			{
			AbleItem( Item( iCCDBoldCheck + i ), true );
			}
		
		// set checks according to the current style
		// (has to happen after enabling, but before disabling :<)
		SetValue( Item( iCCDBoldCheck + i ),
					( curStyles[curStyleIdx].face & ( 1U << i ) ) ? 1 : 0 );
		
		// bubbles can't have alt. styles
		if ( kMsgBubble == curStyleIdx || kMsgFriendBubble == curStyleIdx )
			{
			AbleItem( Item( iCCDBoldCheck + i ), false );
			}
		}
	
	// next, do the opacity menu
	if ( kMsgBubble == curStyleIdx || kMsgFriendBubble == curStyleIdx )
		{
		AbleItem( Item( iCCDOpacityMenu ), true );
		int opacity = (curStyleIdx == kMsgBubble) ?
							curBubbleBlitter : curFriendBubbleBlitter;
		SetValue( Item( iCCDOpacityMenu ), OpacityToItem( opacity ) );
		}
	else
		{
		SetValue( Item( iCCDOpacityMenu ), mOpacityNormal );
		
		AbleItem( Item( iCCDOpacityMenu ), false );
		}
	
#if USE_SPEECH
	// next, do the Speak checkbox
	// for the nonce, only certain friend messages are speakable
	switch ( curStyleIdx )
		{
		case kMsgFriendLogon:
		case kMsgFriendLogoff:
		case kMsgFriendObit:
		case kMsgHost:
			SetValue( Item( iCCDSpeakCheck ), curSpeaking[ curStyleIdx ] ? 1 : 0 );
			AbleItem( Item( iCCDSpeakCheck ), gSpeechAvailable );
			break;
		
		default:
			SetValue( Item( iCCDSpeakCheck ), 0 );
			AbleItem( Item( iCCDSpeakCheck ), false );
		}
#endif  // USE_SPEECH
	
	// lastly, force a redraw of the dynamic items
	HIViewRef vu = GetItem( Item( iCCDSampleText ) );
	__Verify_noErr( HIViewSetNeedsDisplay( vu, true ) );
	vu = GetItem( Item( iCCDColorSwatch ) );
	__Verify_noErr( HIViewSetNeedsDisplay( vu, true ) );
}


/*
**	ColorChoiceDlg::SaveInfo()
**
**	copy user's choices to global prefs
*/
void
ColorChoiceDlg::SaveInfo()
{
	for ( int i = kMsgDefault; i < kMsgLastMsg; ++i )
		{
		gPrefsData.pdStyles[ i ] = curStyles[ i ];
		gPrefsData.pdSpeaking[ i ] = curSpeaking[ i ];
		}
	
	gPrefsData.pdBubbleBlitter = curBubbleBlitter;
	gPrefsData.pdFriendBubbleBlitter = curFriendBubbleBlitter;
}

#pragma mark -


/*
**	Sanitize1TextStyle
**
**	because it's not yet safe to allow arbitrary text sizes, fonts, etc.
*/
static void
Sanitize1TextStyle( int ii )
{
	CLStyleRecord * p = &gPrefsData.pdStyles[ ii ];
	
	p->font = 1; // applFont;
	
	switch ( ii )
		{
		case kMsgBubble:
		case kMsgFriendBubble:
			p->size = 9;
			break;
		
		case kMsgSpeech:
		case kMsgFriendSpeech:
		case kMsgMySpeech:
		case kMsgThoughtMsg:
			p->size = 12;
			break;
		
		case kMsgMonoStyle:
			p->size = 10;
			p->font = 4; // kFontIDMonaco;
			break;
		
		default:
			p->size = 10;
			break;
		}
}


/*
**	Set1TextStyleFromDict()
**
**	init 1 CLStyleRecord from a CFDictionary
*/
static void
Set1TextStyleFromDict( int idx, CFDictionaryRef dr, bool bIsPref )
{
	CLStyleRecord * p = &gPrefsData.pdStyles[ idx ];
	
	// set initial default defaults
	// (unless we're applying saved styles from the prefs: in that case, we don't want
	// to overwrite the "regular" defaults that came from the in-app plist file.)
	if ( not bIsPref )
		{
		p->font = 1;	// applFont
		p->face = normal;
		p->size = 10;
		p->color.SetBlack();
		}
	
	// pick up fontID...
	const void * vp = CFDictionaryGetValue( dr, TS_FontID );
	if ( vp && CFNumberGetTypeID() == CFGetTypeID( vp ) )
		{
		CFNumberGetValue( static_cast<CFNumberRef> ( vp ),
			kCFNumberSInt16Type, &p->font );
		}
	
	// but if there's a font _name_, override fontID
	vp = CFDictionaryGetValue( dr, TS_FontName );
	if ( vp && CFStringGetTypeID() == CFGetTypeID( vp ) )
		{
		CFStringRef cs = static_cast<CFStringRef>( vp );
		ATSFontFamilyRef font = ATSFontFamilyFindFromName( cs, kNilOptions );
		
		// ATS family #s are 32-bits; QD fontIDs were 16
		if ( font && font != UINT_MAX && font < SHRT_MAX )
			p->font = font;
		}
	
	// font face (i.e. bold/italic/etc.) ...
	vp = CFDictionaryGetValue( dr, TS_Face );
	if ( vp && CFNumberGetTypeID() == CFGetTypeID( vp ) )
		{
		CFNumberGetValue( static_cast<CFNumberRef> ( vp ),
			kCFNumberSInt16Type, &p->face );
		}
	
	// font size...
	vp = CFDictionaryGetValue( dr, TS_Size );
	if ( vp && CFNumberGetTypeID() == CFGetTypeID( vp ) )
		{
		CFNumberGetValue( static_cast<CFNumberRef> ( vp ),
			kCFNumberSInt16Type, &p->size );
		}
	
	// ... and the text color
	vp = CFDictionaryGetValue( dr, TS_Color );
	if ( vp && CFArrayGetTypeID() == CFGetTypeID( vp ) )
		{
		CFArrayRef a = static_cast<CFArrayRef>( vp );
		
		// collect all 4 components. Maybe someday the alpha will be useful
		float c_comp[ 4 ];
		for ( int cc = 0; cc < 4; ++cc )
			{
			c_comp[ cc ] = 0;
			
			vp = CFArrayGetValueAtIndex( a, cc );
			if ( vp && CFNumberGetTypeID() == CFGetTypeID( vp ) )
				{
				CFNumberGetValue( static_cast<CFNumberRef>( vp ), kCFNumberFloatType,
					&c_comp[ cc ] );
				}
			}
		
		p->color.rgbRed		= 0xFFFF * c_comp[ 0 ];
		p->color.rgbGreen	= 0xFFFF * c_comp[ 1 ];
		p->color.rgbBlue	= 0xFFFF * c_comp[ 2 ];
//		p->color.rgbAlpha	= 0xFFFF * c_comp[ 3 ];
		}
	
	// and the 'speaking'
	gPrefsData.pdSpeaking[ idx ] = false;
	vp = CFDictionaryGetValue( dr, TS_Speak );
	if ( vp && CFBooleanGetTypeID() == CFGetTypeID( vp ) )
		{
		CFBooleanRef b = static_cast<CFBooleanRef>( vp );
		gPrefsData.pdSpeaking[ idx ] = CFBooleanGetValue( b );
		}
	
	// prophylaxis vs meddlesome users who diddle around in the app's Resources folder
	Sanitize1TextStyle( idx );
}


/*
**	InitTextStyles()
**
**	pre-load the default text styles, prior to applying any customizations in the user's prefs.
**	This gets called before we've even opened the prefs file.
**	The prototypical text styles are read from a property list;
**	we parse them into 'sStyleArray', and also save them in the gPrefs.
**	Moreover, this sets up the sStyleNames array.
**	Only sets styles kMsgDefault thru kMsgTimeStamp.
*/
void
InitTextStyles()
{
	if ( sStyleArray )
		return;		// already loaded
	
	// load the styles array from the plist
	CFURLRef url = CFBundleCopyResourceURL( CFBundleGetMainBundle(),
						TEXT_STYLES_FILE, CFSTR("plist"), CFSTR("") );
	__Check( url );
	if ( url )
		{
		// read the XML text from the bundled resource file
		SInt32 err = noErr;
		CFDataRef dr = nullptr;
		bool got = CFURLCreateDataAndPropertiesFromResource( kCFAllocatorDefault, url,
						&dr,		// just the file-contents data...
						nullptr, nullptr,	// ... no properties
						&err );
		__Check_noErr( err );
		__Check( dr );
		CFRelease( url );
		
		// convert the XML into an in-memory plist (which is a dictionary)
		CFPropertyListRef plr = nullptr;
		if ( got && dr )
			{
			plr = CFPropertyListCreateFromXMLData( kCFAllocatorDefault,
						dr, kCFPropertyListImmutable, nullptr );
			CFRelease( dr );
			}
		
		// extract the "Styles" array from that dictionary
		if ( plr )
			{
			if ( CFDictionaryGetTypeID() == CFGetTypeID( plr ) )
				{
				CFDictionaryRef dictRef = static_cast<CFDictionaryRef>( plr );
				
				const void * ary = CFDictionaryGetValue( dictRef, TEXT_STYLES_KEY );
				if ( ary
				&&   CFArrayGetTypeID() == CFGetTypeID( ary ) )
					{
					sStyleArray = CFArrayCreateCopy( kCFAllocatorDefault,
										static_cast<CFArrayRef>( ary ) );
					}
				}
			
			CFRelease( plr );
			}
		}
	
	// parse each array entry into the prefs
	if ( sStyleArray )
		{
		CFIndex nn = CFArrayGetCount( sStyleArray );
		__Check( kMsgLastMsg == nn );
		
		CFMutableArrayRef names = CFArrayCreateMutable( kCFAllocatorDefault,
									nn, &kCFTypeArrayCallBacks );
		__Check( names );
		
		for ( int ii = kMsgDefault; ii < kMsgLastMsg; ++ii )
			{
			if ( ii > nn )
				break;
			
			const void * vp = CFArrayGetValueAtIndex( sStyleArray, ii );
			if ( vp && CFDictionaryGetTypeID() == CFGetTypeID( vp ) )
				{
				CFDictionaryRef d1 = static_cast<CFDictionaryRef>( vp );
				
				Set1TextStyleFromDict( ii, d1, false );
				
				if ( names )
					{
					vp = CFDictionaryGetValue( d1, TS_Name );
					if ( vp && CFStringGetTypeID() == CFGetTypeID( vp ) )
						CFArrayAppendValue( names, vp );
					}
				}
			}
		
		if ( names )
			{
			sStyleNames = CFArrayCreateCopy( kCFAllocatorDefault, names );
			CFRelease( names );
			
			__Check( CFArrayGetCount( sStyleNames ) == nn );
			}
		}
}


/*
**	SetUpStyle()
**
**	choose the right text style
*/
void
SetUpStyle( MsgClasses msgClass, CLStyleRecord * rec )
{
	*rec = gPrefsData.pdStyles[ msgClass ];
}


#pragma mark Prefs

// are we using CFPreferences to store app prefs?	[Not yet]
#define USE_CF_PREFERENCES		0

#if USE_CF_PREFERENCES

/*
**	CreateArrayFromColor()
*/
static CFArrayRef
CreateArrayFromColor( const DTSColor& color )
{
	// create the color array: r, g, b, and [someday] alpha
	const void * c[ 4 ];
	float comp;
	
	comp = color.rgbRed / 65535.0F;
	c[ 0 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberFloatType, &comp );
	comp = color.rgbGreen / 65535.0F;
	c[ 1 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberFloatType, &comp );
	comp = color.rgbBlue / 65535.0F;
	c[ 2 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberFloatType, &comp );
	comp = 0;	// placeholder alpha
	c[ 3 ] = CFNumberCreate( kCFAllocatorDefault, kCFNumberFloatType, &comp );
	
	CFArrayRef result = CFArrayCreate( kCFAllocatorDefault, c, 4, &kCFTypeArrayCallBacks );
	__Check( result );

	// now that the array has its own retain on those 4 numbers, it's safe to remove ours
	for ( int jj = 0; jj < 4; ++jj )
		if ( c[ jj ] )
			CFRelease( c[ jj ] );
	
	return result;
}


/*
**	CreateDictForTextStyle()
*/
static CFDictionaryRef
CreateDictForTextStyle( const CLStyleRecord * p, bool bIsSpoken )
{
	CFDictionaryRef result = nullptr;
	
	// style-dict keys
	const void * styleKeys[] = {
		TS_FontID,
		TS_Face,
		TS_Size,
		TS_Color,
		TS_Speak,
		TS_FontName,
		};
	const int kNumKeys = sizeof styleKeys / sizeof styleKeys[ 0 ];
	
	enum {	// indexes corresponding to the above keys
		tsFont = 0,
		tsFace,
		tsSize,
		tsColor,
		tsSpeak,
		tsFName,
		};
	
	const void * styleValues[ kNumKeys ];
	styleValues[ tsFont ] = CFNumberCreate( kCFAllocatorDefault,
								kCFNumberSInt16Type, &p->font );
	styleValues[ tsFace	] = CFNumberCreate( kCFAllocatorDefault,
								kCFNumberSInt16Type, &p->face );
	styleValues[ tsSize ] = CFNumberCreate( kCFAllocatorDefault,
								kCFNumberSInt16Type, &p->size );
	styleValues[ tsFName ] = nullptr;  // for now
	if ( false )
		{
		// put in the name?
		CFStringRef fname = nullptr;
		if ( noErr == ATSFontFamilyGetName( p->font, kNilOptions, &fname )
		&&   fname )
			{
			styleValues[ tsFName ] = fname;
			}
		}
	
	styleValues[ tsColor ] = CreateArrayFromColor( p->color );
	
	// use the 2 predefined CFBoolean values for this setting
	// (and remember not to CFRelease them at the end!)
	styleValues[ tsSpeak ] = bIsSpoken ? kCFBooleanTrue : kCFBooleanFalse;
	
	// make the style dictionary for this entry
	result = CFDictionaryCreate( kCFAllocatorDefault,
					styleKeys,
					styleValues,
					styleValues[ tsFName ] ? kNumKeys : kNumKeys - 1,
					&kCFTypeDictionaryKeyCallBacks,
					&kCFTypeDictionaryValueCallBacks );
	
	// release the various constituent CFNumbers (but not the booleans)
	for ( int jj = 0; jj < kNumKeys; ++jj )
		if ( jj != tsSpeak && styleValues[ jj ] )
			CFRelease( styleValues[ jj ] );
	
	return result;
}


/*
**	SaveTextStylesToPrefs()
**
**	store the user's text styles in CFPreferences
*/
void
SaveTextStylesToPrefs()
{
	const void * styleDicts[ kMsgMaxStyles ];
	
	// for each style...
	for ( int ii = 0; ii < kMsgMaxStyles; ++ii )
		{
		// make the dictionary for this style
		styleDicts[ ii ] = CreateDictForTextStyle( &gPrefsData.pdStyles[ ii ],
								gPrefsData.pdSpeaking[ ii ] );
		}
	
	// now that we have dictionaries for all the individual styles,
	// pack them up into a single array. 
	CFArrayRef styleArray = CFArrayCreate( kCFAllocatorDefault,
				styleDicts, kMsgMaxStyles, &kCFTypeArrayCallBacks );
	
	// and release the individual dictionaries
	for ( int jj = 0; jj < kMsgMaxStyles; ++jj )
		{
		if ( styleDicts[ jj ] )
			CFRelease( styleDicts );
		}
	
	// that's our pref!
	Prefs::Set( pref_TextStyles, styleArray );
	if ( styleArray )
		CFRelease( styleArray );
}


/*
**	GetTextStylesFromPrefs()
*/
void
GetTextStylesFromPrefs()
{
	CFTypeRef tr = Prefs::Copy( pref_TextStyles );
	if ( not tr || CFArrayGetTypeID() != CFGetTypeID( tr ) )
		return;
	
	CFArrayRef ar = static_cast<CFArrayRef>( tr );
	CFIndex nStyles = CFArrayGetCount( ar );
	if ( nStyles > kMsgMaxStyles )
		nStyles = kMsgMaxStyles;
	
	for ( int jj = 0; jj < nStyles; ++jj )
		{
		CFTypeRef t2 = CFArrayGetValueAtIndex( ar, jj );
		if ( not t2 || CFDictionaryGetTypeID() != CFGetTypeID( t2 ) )
			continue;
		
		CFDictionaryRef dr = static_cast<CFDictionaryRef>( t2 );
		Set1TextStyleFromDict( jj, dr, true );
		}
	
	CFRelease( ar );
}
#endif  // USE_CF_PREFERENCES

