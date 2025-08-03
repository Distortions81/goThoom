/*
**	GameWin_cl.cp		ClanLord
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
#include "Frame_cl.h"
#if USE_STYLED_TEXT
# include "LaunchURL_cl.h"
#endif
#include "Macros_cl.h"
#include "Movie_cl.h"
#include "Night_cl.h"
#include "SendText_cl.h"
#include "Shadows_cl.h"
#include "Speech_cl.h"
#include "TuneHelper_cl.h"

#ifdef USE_OPENGL
# include "OpenGL_cl.h"
# include <OpenGL/glu.h>
# ifdef OGL_SOFTWARE_CLIENT_STORAGE_LIGHTMAP
#  include <aglRenderers.h>
# endif	// OGL_SOFTWARE_CLIENT_STORAGE_LIGHTMAP
#endif	// USE_OPENGL

// undefine these to turn off the feature(s)
#define	TRANSLUCENT_HEALTH_BARS
//#define TRANSLUCENT_HAND_ITEMS


//#define OVAL_MOBILE_SHADOWS
#define MOBILE_SHADOWS
// #define PLANE_ZERO_SHADOWS

// hilite mode for selected player icon
#define NEW_OVAL_HILITE


// placement and color style for the translucent health bars, if they're shown.
// Players can now set them in the Advanced Prefs dialog.

enum
	{
	HB_Placement_Bottom = 0,		// horizontally across bottom
	HB_Placement_LowerLeft,			// stacked in lower-left
	HB_Placement_LowerRight,		// similar
	HB_Placement_UpperRight,		// similar
	
	HB_Color_Fixed = 0,				// separate, constant color for each bar
	HB_Color_Variable				// bars change color as the stat does
	};


/*
**	The draw state message has the following format:
**	(warning, this diagram sometimes lags behind reality. trust the code first.)
**
**	Key:
**	|  = repeated entry, aligned under controlling count field
**	?  = "optional" entry; presence depends on prior fields/flags
**	*  = variable length (implicit for 'cs')
**
**	sx = signed,	ux = unsigned
**	xb = int8,		xs = int16,		xl = int32
**	cs = c string
**
**	header
**	------
**	us		message tag
**	ub		ack command number
**	ul		frame
**	ul		resend frame
**
**	descriptors
**	-----------
**	ub		descriptor count
**	| ub	index
**	| ub	type (unknown, plyr, monster, npc, etc.)
**	| us	pict id
**	| cs	name
**	| ub	color count
**	| | ub	colors
**
**	frame
**	-----
**	ub		hp value
**	ub		hp max
**	ub		sp value
**	ub		sp max
**	ub		balance value
**	ub		balance max
**	ub		lighting flags
**	ub		picture count
**	?ub		count of repeated pictures	(only if picture count == 255)
**	?ub		count of new pictures	 	(only if picture count == 255)
**	:			// NOTE: pictIDs and coords are now packed, hence no longer
**	:			// align to 8-bit boundaries. See UnspoolFramePicture()
**	| us	pict id	(actually 14 bits)	(# copies = picture_count, or new_picture_count)
**	| ss	horz	(11 bits)
**	| ss	vert	(11 bits)
**	ub		num mobiles
**	| ub	descriptor index
**	| us	state
**	| ss	vert
**	| ss	horz
**	| ub	colors
**
**	state
**	-----
**	cs		info text
**	ub		bubble count
**	| ub		desc index
**	| ub		type + flags
**	| ?ub		language flags			(only if (type & kBubbleNotCommon))
**	| ?ss		horz offset				(only if (type & kBubbleFar))
**	| ?ss		vert offset				(only if (type & kBubbleFar))
**	| cs		text
**	ub		sound count
**	| us		sound id
**	ub		inventory command
**	if ( command is not 'multiple' ) {
**		if ( command is 'full' ) {
**			ub		inventory count
**			ub*		equip flags				(bitmap, sent as ceil(count/8) bytes)
**			| us		item id
**		} elsif ( command is not 'none' ) {
**			us		item id
**			?ub		item index				(only if kInvCmdIndex flag set)
**			?cs		item custom name		(only if command is Add, AddEquip, or Name)
**		} endif
**	} else {
**		ub		subcommand count
**		| ub		inventory subcommand
**		| cmd_data	(identical to not-multiple case, above)
**	} endif
*/


/*
**	Entry Routines
*/
/*
void	CreateCLWindow( DTSRect * pos );
void	RedrawCLWindow();
void	GetGWMouseLoc( DTSPoint * pt );
void	ExtractImportantData();
void	ShowInfoText( const char * text );
void	ChangeWindowSize();
void	SetBackColors();
void	UpdateCommandsMenu( const char * );	// v83

void	TouchHealthBars();			// strictly temporary (har, har)
bool	MovePlayerToward( int quadrant, bool fast, bool stopIfBalance );
*/

// FIXME: we should separate the playfield-drawing stuff into its own source file

/*
**	Global Variables
*/
#ifdef	HANDLE_MUSICOMMANDS
CTuneQueue		gTuneQueue;
#endif
CCLFrame *		gFrame;
int				gFastDrawLimit = 180;		// need to find the best value for this
int				gSlowDrawLimit = 245;		// should be less than 250
// (those draw-time limits are laughable now, but v. important
// back in the days of 68k and non-accelerated QuickDraw)

/*
**	Definitions
*/
const int	kCLDepth			= 8;		// bits per pixel
const int	kLineHeight			= 13;		// line height for the text field
const int	kTextDelayLength	= 8 * kFramesPerSecond;
#ifdef OGL_USE_TEXT_FADE
const int	kTextFadeLength		= 1 * kFramesPerSecond;
#endif	// OGL_USE_TEXT_FADE

const int	kLargeLayoutID		= 0;
//const int	kSmallLayoutID		= 1;

const int	kTextAscent			= 10;
const int	kTextDescent		= 2;
const int	kTextLineHeight		= kTextAscent + kTextDescent + 0;
const int	kTextInsetHorz		= 14;
const int	kTextInsetVert		= 4;
const int	kTextSmallWidth		= 56;
const int	kTextMediumWidth	= 90;
const int	kTextLargeWidth		= 136;
const int	kBubblePadHorz		= 10;
const int	kBubblePadVert		= 3;
const int	kBubbleSmallHeight	= 33;
const int	kBubbleSmallWidth	= 84;
const int	kBubbleMediumHeight	= 43;
const int	kBubbleMediumWidth	= 116;
const int	kBubbleLargeHeight	= 53;
const int	kBubbleLargeWidth	= 164;
const int	kTextViewHeight		= kTextAscent + kTextDescent;
const int	kTextViewWidth		= kTextLargeWidth * 5;
const int	kTextUpperOffset	= 14;
const int	kTextLowerOffset	= -5;
const int	kTextLeftOffset		= 6;
const int	kTextRightOffset	= 6;

const int	kPictDefQualityMode	= kPictDefFlagEffect;

const char	kBEPPChar			= '\xC2';	// "not sign" character

enum
	{
	kDrawBubbleNormal,
	kDrawBubbleNamePrefix,

	kStateSizeHi = 0,
	kStateSizeLo,
	kStateData,
	kStateHorked
	};

#define F_PI		float( M_PI )

#if defined( MULTILINGUAL )

// This is used by the multilingual think bubble code
// The same BEPP definitions are in Socks' constants/backendprotocol.h
// and are used by Arindal's lang_think() custom think-command.

enum
	{
	kBubbleThink_none = 0,
	kBubbleThink_think,
	kBubbleThink_thinkto,
	kBubbleThink_thinkclan,
	kBubbleThink_thinkgroup
	};

#define kBEPPCharacter					"\xC2"	// BEPP lead-in character (not-sign)
#define kBEPPThinkBubble_think			kBEPPCharacter "t_th"
#define kBEPPThinkBubble_thinkto		kBEPPCharacter "t_tt"
#define kBEPPThinkBubble_thinkclan		kBEPPCharacter "t_tc"
#define kBEPPThinkBubble_thinkgroup		kBEPPCharacter "t_tg"

#endif  // MULTILINGUAL

// CFPreferences key for ShowDrawTimes() / gShowDrawTime
#define kShowDrawTimePrefKey			CFSTR("DebugShowDrawTimes")


/*
**	TextView class
*/
class TextView : public DTSOffView
{
protected:
	DTSImage		tvImage;
	const char *	tvText;
	bool			tvFriends;
	
public:
	// constructor
			TextView() : tvText() {}
	
	// overloaded routines
	virtual void	DoDraw();
	
	// interface routines
	void	InitTV();
	void	SetText( const char * t, bool f ) { tvText = t; tvFriends = f; }
	const DTSImage *	GetTVImage() const { return &tvImage; }
};


#ifdef TRANSLUCENT_HEALTH_BARS
class TransludentBarView : public DTSOffView
{
protected:
	int					mValue;
	int					mMax;
	const DTSColor *	mColor;
	
public:
	TransludentBarView() : mColor() {}
	
	// overloaded routines
	virtual void	DoDraw();
	
	void	DrawTransBar( int value, int max, const DTSColor * color )
		{
		mValue = value;
		mMax   = max;
		mColor = color;
		Draw();
		}
	
	void	Draw1TranslucentBar( int mValue, int mMax, const DTSColor * mColor,
				ushort alpha, const DTSRect& viewBounds );
};
#endif  // TRANSLUCENT_HEALTH_BARS


/*
**	PictureQueue class
*/
const int kMaxPicQueElems	= 254 + 254; // max of "pict-agains" plus new ones
struct PictureQueue
{
	ImageCache *	pqCache;
	int				pqPlane;
	int				pqHorz;
	int				pqVert;
};


/*
**	CLOffView class
*/
class CLOffView : public DTSOffView
{
public:
#ifdef USE_OPENGL
				CLOffView() :
					nightList( 0 ),
					nightListLevel( 0 ),
					nightListRedshift( -1.0f ) {}
	virtual		~CLOffView() {}
#endif	// USE_OPENGL
	
	DTSImage			offBufferImage;
	bool				offValid;
	
#ifdef	TRANSLUCENT_HEALTH_BARS
	// number of ticks the full bars are still displayed
	// before disappearing (gossip mode)
	static const int kStatusBarShowDelay	= 5 * 60;		// ticks
	static const uint kStatusBarFull		= (255 * 255) + (255 * 255) + (255 * 255);
	
	ulong				statusBarTouch;
	DTSImage			offStatusBar;
	TransludentBarView	offStatusBarView;
#endif
	
	static const int KMovieSignBlinkDelay	= 60;
	
	ulong				offMovieSignBlink;
	bool				offMovieSignShown;
	ImageCache *		offSmallBubbleImage;
	ImageCache *		offMediumBubbleImage;
	ImageCache *		offLargeBubbleImage;
	TextView			offTextView;
	
	// overloaded routines
	virtual void	DoDraw();
	
	// interface routines
	void	DrawFirstTime();
	void	DrawValid();
	
	void	DrawQueuedPictures( int maxPlane );
	void	RebuildNightList( const DTSRect& dst );
	void	Draw1Picture( const PictureQueue * picture );
	void	SortMobiles();
	void	DrawMobileShadows();
	void	Draw1MobileShadow( const DSMobile * mobile );
#ifdef USE_OPENGL
	void	DrawSelectionHilite();	// overload
#endif	// USE_OPENGL
	void	DrawSelectionHilite( DTSRect box, int mobileSize, int state );
	void	DrawNames();
	void	Draw1Name( const DSMobile * mobile );
#ifdef AUTO_HIDENAME
	void	DrawBlendedName( const DTSRect * box, int vis, const DTSColor * bkClr,
				const DTSColor * txClr, int style, const char * name );
#endif
	void	Draw1InjuryBar( const DSMobile * mobile, int friends );
	void	DrawMobiles();
	void	Draw1Mobile( const DSMobile * player );
	void	DrawBubbles();
	void	Draw1Bubble( DescTable * desc, int mode );
	void	DrawHandObjects();
	void	DrawBar( const DTSRect * box, int value, int max );
#ifdef	TRANSLUCENT_HEALTH_BARS
	void	DrawTranslucentHealthBars();
#endif
#ifdef	TRANSLUCENT_HAND_ITEMS
	void	DrawTranslucentHandObjects();
#endif
	void	DrawMovieMode();
	void	DrawMovieProgress( const DTSRect& );
	void	DrawFieldInactive();

#ifdef OGL_SHOW_DRAWTIME
	void	ShowDrawTime();
#endif  // OGL_SHOW_DRAWTIME
	
#ifdef USE_OPENGL
			// normally we'd restrict ourselves to the ascii characters,
			// but CL players type all kinds of weird stuff (in MacRoman)
			enum { kFontArraySize = 256 };
	
	// this SHOULD be protected...
	void	CreateOGLFontDisplayLists( AGLContext ctx );
	void	DeleteOGLObjects();
	void	DeleteNonOGLObjects();
	
protected:
	GLuint	Make1FontList( AGLContext ctx, FMFontFamily fontID,
				int fsize, Style face );
	
	GLuint geneva9NormalListBase;
	GLuint geneva9BoldListBase;
	GLuint geneva9ItalicListBase;
	GLuint geneva9BoldItalicListBase;
	GLuint geneva9UnderlineListBase;
	GLuint geneva9BoldUnderlineListBase;
	GLuint geneva9ItalicUnderlineListBase;
	GLuint geneva9BoldItalicUnderlineListBase;
	GLuint nightList;
	GLuint nightListLevel;
	GLfloat nightListRedshift;
#endif	// USE_OPENGL
};


/*
**	CLWindow class
*/
class CLWindow : public DTSWindow, public CarbonEventResponder
{
public:
	CLOffView		winOffView;
	TextFieldType	winTextField;
	bool			winInRedraw;	// true iff redrawing data from new packet;
									// false for updtEvts
	
	// overloaded routines
	virtual void			DoDraw();
	virtual void			Activate();
	virtual void			Deactivate();
	virtual bool			Click( const DTSPoint& loc, ulong time, uint modifiers );
	virtual bool			KeyStroke( int ch, uint modifiers );
	virtual void			Idle();
	virtual bool			DoCmd( long menuCmd );
//	virtual void			Size( DTSCoord width, DTSCoord height );
	
	DTSError				InstallEventHandlers();
	
	// handle mouse stuff: wheels, clicks...
	OSStatus				DoMouseDownEvent( const CarbonEvent& );
	OSStatus				DoWheelEvent( const CarbonEvent& );
	OSStatus				DoWindowEvent( CarbonEvent& );
	
	// carbon-event vector
	virtual OSStatus		HandleEvent( CarbonEvent& );
};


struct WordTable
{
	const char *	text;
	short			startPos;
	short			stopPos;
	short			length;
	short			pad;
};


/*
**	Internal Routines
*/
static void				DynamicInitCLWindow();
static DTSError			InitLayout();
static WordTable *		GetWordTable( const char * text );
static WordTable *		AdjustWordTable( WordTable * words );
static int				InitLineTable( WordTable * lines, WordTable * words, int width );
static WordTable *		Init1Line( WordTable * lines, WordTable * words, int width );
static void				ChooseBubblePosition( const DTSImage * image, DescTable * desc );
static int				CalcBubblePosValue( const DTSImage * image, DescTable * desc, int pos );
static void				PositionBubble( DTSRect *, const DTSImage *, const DescTable *, int );
static int				PinBubble( DTSRect * dst, int pos );
static void				ChooseThoughtPosition( const DTSImage * image, DescTable * desc );
static void				ExtractDescriptors( const CCLFrame * ds );
static void				ExtractFramePictures( const CCLFrame * ds );
static void				Queue1Picture( int nnn, DTSKeyID pictID, int horz, int vert );
static void				ExtractFrameMobiles( const CCLFrame * ds );
static void				ExtractStateData( const CCLFrame * ds, int lastAckFrame, int resend );
static void				HandleStateData();
static uchar *			HandleInfoText( uchar * ptr );
#if defined( MULTILINGUAL )
static bool				CheckRealLanguageId( const char * start, size_t len );
#endif	// MULTILINGUAL
static const uchar *	Handle1Bubble( const uchar * ptr );
static const uchar *	Handle1Sound( const uchar * ptr );
static const uchar *	HandleInventory( const uchar * ptr );
static const uchar *	Handle1CustomName( DTSKeyID id, int index, const uchar * ptr );
static bool				CheckLudification( const char * start, size_t len);
#ifdef CL_DO_WEATHER
static bool				CheckWeather( const char * inCmd, size_t inLen );
#endif	// CL_DO_WEATHER
static void				StringCopySafeNoDupSpaces( char * dst, const char * src, size_t sz );
static void				SelectiveBlit( DTSOffView * view, const DTSImage * srci,
							const DTSRect * srcr, const DTSRect * dstr, int which );
static void				DoNotification( const char * senderName, const char * format,
							const char * thought );
#if USE_WHISPER_YELL_LANGUAGE
static void				FillUnknownLanguage( int t, int lang, const char * name, char * dest );
#else
static void				FillUnknownLanguage( int lang, const char * name, char * dest );
#endif	// USE_WHISPER_YELL_LANGUAGE
static void				LoadBubbleImages();


/*
**	Internal Variables
*/
static NMRec			gNotif;					// our notification queue element
static int				gMovePlayerQuadrant;	// what direction to move, from macros
static bool				gNotificationActive;	// is there an OS notification active?
//static bool			gMoveStopIfBalance;		// to be used later (har, har)
static bool				gSpecialTedFriendMode;	// self text bubbles not automatically friendly
static bool				gFlipAllBubbles;		// April Fool's Day mode


#ifdef USE_OPENGL	// needed in OpenGL_cl.cpp
Layout					gLayout;
DTSPoint				gFieldCenter;
static int				gCachedCircularLightmapElementRadius = -1;
#else
static Layout			gLayout;
static DTSPoint			gFieldCenter;
#endif	// USE_OPENGL
static int				gNumLines;
static uchar			gCharWidths[ 256 ];
static DTSColor			gBackColorTable[ kColorCodeBackCount ];
static DTSColor			gTextColorTable[ kColorCodeTextCount ];

#ifdef AUTO_HIDENAME
static DescTable *		gHoveringOnPlayer;
static int				gHoveringSince;
#endif

static DSMobile *		gThisMobile;
static int				gPicQueCount;
static PictureQueue *	gPicQueStart;
static PictureQueue		gPictureQueue[ kMaxPicQueElems ];

#ifdef OGL_SHOW_DRAWTIME
static float			gOGLDrawTime;
static bool				gShowDrawTime;
#endif	// OGL_SHOW_DRAWTIME


const RGBColor
sLangColors[ kBubbleNumLanguages ] =
{
		{ 0xC0C0, 0x6060, 0x6060 },		// 0 halfing
		{ 0xC0C0, 0xA8A8, 0x6060 },		// 1 sylvan
		{ 0x9090, 0xC0C0, 0x6060 },		// 2 people
		{ 0x6060, 0xC0C0, 0x7878 },		// 3 thoom
		{ 0x6060, 0xC0C0, 0xC0C0 },		// 4 dwarf
		{ 0x6060, 0x7878, 0xC0C0 },		// 5 ghorak
		{ 0x9090, 0x6060, 0xC0C0 },		// 6 ancient
		{ 0xC0C0, 0x6060, 0xA8A8 },		// 7 magic
		{ 0xFFFF, 0xFFFF, 0xFFFF },		// 8 new 'common' == white, but color not used
		{ 0x5050, 0x5050, 0x5050 },		// 9 thieves -- grayish?
		{ 0x4040, 0x2020, 0xE0E0 },		// 10 mystic-spirit
#ifdef ARINDAL
		{ 0xC0C0, 0x2020, 0x1010 }, 	// 11 monster
		{ 0x9090, 0x9090, 0x7070 },		// 12 unknown
		{ 0x5050, 0x7070, 0x1010 },		// 13 orga
		{ 0x9090, 0xA0A0, 0xB0B0 },		// 14 sirrush
		{ 0xE0E0, 0x9090, 0x4040 },		// 15 azcatl
		{ 0x8080, 0xB0B0, 0xFFFF }		// 16 lepori
#endif // ARINDAL
};

const char * const
gLanguageTableShort[] =
{
	TXTCL_LANGTABLE1_GIBBERISH,	// "gibberish",
	TXTCL_LANGTABLE1_HALFLING,	// "Halfling",
	TXTCL_LANGTABLE1_SYLVAN,	// "Sylvan",
	TXTCL_LANGTABLE1_PEOPLE,	// "People",
	TXTCL_LANGTABLE1_THOOM,		// "Thoom",
	TXTCL_LANGTABLE1_DWARVEN,	// "Dwarven",
	TXTCL_LANGTABLE1_GHORAK_ZO,	// "Ghorak Zo",
	TXTCL_LANGTABLE1_ANCIENT,	// "Ancient",
	TXTCL_LANGTABLE1_MAGIC,		// "magic",
	TXTCL_LANGTABLE1_COMMON,	// "common",
	TXTCL_LANGTABLE1_CODE,		// "code",		// (?) thieves' tongue
	TXTCL_LANGTABLE1_LORE,		// "lore"		// (?) mystic argot
#ifdef ARINDAL
	TXTCL_LANGTABLE1_MONSTER,	// "Monster",
	TXTCL_LANGTABLE1_UNKNOWN,	// "unknown",
	TXTCL_LANGTABLE1_ORGA,		// "Orga",
	TXTCL_LANGTABLE1_SIRRUSH,	// "Sirrush",
	TXTCL_LANGTABLE1_AZCATL,	// "Azcatl",
	TXTCL_LANGTABLE1_LEPORI 	// "Lepori",
#endif  // ARINDAL
};
const int kNumLanguages = sizeof( gLanguageTableShort ) / sizeof( char * );

const char * const
gLanguageTableLong[] =
{
	TXTCL_LANGTABLE2_GIBBERISH,	// "the language of the mad",
	TXTCL_LANGTABLE2_HALFLING,	// "the Halfling language",
	TXTCL_LANGTABLE2_SYLVAN,	// "the Sylvan language",
	TXTCL_LANGTABLE2_PEOPLE,	// "the language of The People",
	TXTCL_LANGTABLE2_THOOM,		// "the Thoom language",
	TXTCL_LANGTABLE2_DWARVEN,	// "the Dwarven language",
	TXTCL_LANGTABLE2_GHORAK_ZO,	// "the Ghorak Zo language",
	TXTCL_LANGTABLE2_ANCIENT,	// "an ancient language",
	TXTCL_LANGTABLE2_MAGIC,		// "the language of magic",
	TXTCL_LANGTABLE2_COMMON,	// "the common language",
	TXTCL_LANGTABLE2_CODE,		// "a secret language",
	TXTCL_LANGTABLE2_LORE,		// "a mystical language"
#ifdef ARINDAL
	TXTCL_LANGTABLE2_MONSTER,	// "the monster language",
	TXTCL_LANGTABLE2_UNKNOWN,	// "an unknown language",
	TXTCL_LANGTABLE2_ORGA,		// "the orga language",
	TXTCL_LANGTABLE2_SIRRUSH,	// "the Sirrush language",
	TXTCL_LANGTABLE2_AZCATL,	// "the Azcatl language",
	TXTCL_LANGTABLE2_LEPORI 	// "the Lepori language",
#endif  // ARINDAL
};

#if USE_WHISPER_YELL_LANGUAGE
enum
{
	kLanguageNormalVerb = 0,
	kLanguageWhisperVerb = 1,
	kLanguageUnknownYell0Verb = 2,
	kLanguageUnknownYell1Verb = 3,
	kLanguageUnknownYell2Verb = 4,
	kLanguageLogYellVerb = 5,
	
	kLanguageVerbTypeRange = 6,
		// FillUnknownLanguage() uses 
		//	sizeof "gLanguageTableShort / sizeof gLanguageTableShort[0]"
	kLanguageRacesRange = sizeof( gLanguageTableShort ) / sizeof( char * ),
	
	kLanguageUnknownYellVerbFirst = kLanguageUnknownYell0Verb,
	kLanguageUnknownYellVerbLast = kLanguageUnknownYell2Verb
};

const char * const
gVerbTable[kLanguageVerbTypeRange][kLanguageRacesRange] =
{
		// normal
	{
	TXTCL_LANGTABLE3_GIBBERISH,	// babbles",
	TXTCL_LANGTABLE3_HALFLING,	// squeaks",
	TXTCL_LANGTABLE3_SYLVAN,	// chirps",
	TXTCL_LANGTABLE3_PEOPLE,	// purrs",
	TXTCL_LANGTABLE3_THOOM,		// hums",
	TXTCL_LANGTABLE3_DWARVEN,	// mutters",
	TXTCL_LANGTABLE3_GHORAK_ZO,	// rumbles",
	TXTCL_LANGTABLE3_ANCIENT,	// chants",
	TXTCL_LANGTABLE3_MAGIC,		// utters",
	TXTCL_LANGTABLE3_COMMON,	// says something",
	TXTCL_LANGTABLE3_CODE,		// gestures",
	TXTCL_LANGTABLE3_LORE,		// incants"
# ifdef ARINDAL
	TXTCL_LANGTABLE3_MONSTER,	// growls",
	TXTCL_LANGTABLE3_UNKNOWN,	// sounds",
	TXTCL_LANGTABLE3_ORGA,		// grunts",
	TXTCL_LANGTABLE3_SIRRUSH,	// hisses",
	TXTCL_LANGTABLE3_AZCATL,	// clacks",
	TXTCL_LANGTABLE3_LEPORI 	// nibbles",
# endif  // ARINDAL
	},
		// whisper
	{
	TXTCL_LANGTABLE4_GIBBERISH,	// babbles softly",
	TXTCL_LANGTABLE4_HALFLING,	// squeaks softly",
	TXTCL_LANGTABLE4_SYLVAN, 	// "chirps softly",	// warbles? coos?
	TXTCL_LANGTABLE4_PEOPLE,	// purrs softly",
	TXTCL_LANGTABLE4_THOOM,		// hums softly",
	TXTCL_LANGTABLE4_DWARVEN,	// mumbles",
	TXTCL_LANGTABLE4_GHORAK_ZO,	// murmurs",
	TXTCL_LANGTABLE4_ANCIENT,	// chants softly",
	TXTCL_LANGTABLE4_MAGIC,		// utters softly",
	TXTCL_LANGTABLE4_COMMON,	// whispers something",
	TXTCL_LANGTABLE4_CODE,		// gestures discreetly",
	TXTCL_LANGTABLE4_LORE,		// incants softly"
# ifdef ARINDAL
	TXTCL_LANGTABLE4_MONSTER,	// growls softly",
	TXTCL_LANGTABLE4_UNKNOWN,	// sounds softly",
	TXTCL_LANGTABLE4_ORGA,		// grunts softly",
	TXTCL_LANGTABLE4_SIRRUSH,	// hisses softly",
	TXTCL_LANGTABLE4_AZCATL,	// clacks softly",
	TXTCL_LANGTABLE4_LEPORI 	// nibbles softly",
# endif  // ARINDAL
	},
		// the unknown yells should "sound like a sound"
		// so "Thoom!" works, but "Bellow!" does not.
		// there are 3 different yells for each language,
		// so things won't sound so repetitious from the same
		// speaker; but some yells are shared between languages,
		// just to add to the confusion

		// unknown yell 0
	{
	TXTCL_LANGTABLE5_GIBBERISH,	// Whoop",
	TXTCL_LANGTABLE5_HALFLING,	// Squeak",
	TXTCL_LANGTABLE5_SYLVAN,	// Chirp",
	TXTCL_LANGTABLE5_PEOPLE,	// Roar",
	TXTCL_LANGTABLE5_THOOM,		// Thoom",
	TXTCL_LANGTABLE5_DWARVEN,	// Humph",
	TXTCL_LANGTABLE5_GHORAK_ZO,	// Roar",
	TXTCL_LANGTABLE5_ANCIENT,	// ...",
	TXTCL_LANGTABLE5_MAGIC,		// ...",
	TXTCL_LANGTABLE5_COMMON,	// Yo",
	TXTCL_LANGTABLE5_CODE,		// ...",
	TXTCL_LANGTABLE5_LORE,		// ..."
# ifdef ARINDAL
	TXTCL_LANGTABLE5_MONSTER,	// ...",
	TXTCL_LANGTABLE5_UNKNOWN,	// ...",
	TXTCL_LANGTABLE5_ORGA,		// Die!",
	TXTCL_LANGTABLE5_SIRRUSH,	// Hissss",
	TXTCL_LANGTABLE5_AZCATL,	// Clickzzz",
	TXTCL_LANGTABLE5_LEPORI 	// Hop hop",
# endif  // ARINDAL
	},
		// unknown yell 1
	{
	TXTCL_LANGTABLE6_GIBBERISH,	// Oh",
	TXTCL_LANGTABLE6_HALFLING,	// Squeal",
	TXTCL_LANGTABLE6_SYLVAN,	// Trill",
	TXTCL_LANGTABLE6_PEOPLE,	// Yowl",
	TXTCL_LANGTABLE6_THOOM,		// Thooom",
	TXTCL_LANGTABLE6_DWARVEN,	// Whoop",
	TXTCL_LANGTABLE6_GHORAK_ZO,	// Ooga",
	TXTCL_LANGTABLE6_ANCIENT,	// ...",
	TXTCL_LANGTABLE6_MAGIC,		// ...",
	TXTCL_LANGTABLE6_COMMON,	// Hey",
	TXTCL_LANGTABLE6_CODE,		// ...",
	TXTCL_LANGTABLE6_LORE,		// ..."
# ifdef ARINDAL
	TXTCL_LANGTABLE6_MONSTER,	// ...",
	TXTCL_LANGTABLE6_UNKNOWN,	// ...",
	TXTCL_LANGTABLE6_ORGA,		// Orga",
	TXTCL_LANGTABLE6_SIRRUSH,	// Hrrrrk",
	TXTCL_LANGTABLE6_AZCATL,	// Bzzzt",
	TXTCL_LANGTABLE6_LEPORI 	// Yip",
# endif  // ARINDAL
	},
		// unknown yell 2
	{
	TXTCL_LANGTABLE7_GIBBERISH,	// Hi",
	TXTCL_LANGTABLE7_HALFLING,	// Yipe",
	TXTCL_LANGTABLE7_SYLVAN,	// Hoot",
	TXTCL_LANGTABLE7_PEOPLE,	// Snarl",
	TXTCL_LANGTABLE7_THOOM,		// Thoooom",
	TXTCL_LANGTABLE7_DWARVEN,	// Beer",
	TXTCL_LANGTABLE7_GHORAK_ZO,	// Hork",
	TXTCL_LANGTABLE7_ANCIENT,	// ...",
	TXTCL_LANGTABLE7_MAGIC,		// ...",
	TXTCL_LANGTABLE7_COMMON,	// ...",
	TXTCL_LANGTABLE7_CODE,		// ...",
	TXTCL_LANGTABLE7_LORE,		// ..."
# ifdef ARINDAL
	TXTCL_LANGTABLE7_MONSTER,	// ...",
	TXTCL_LANGTABLE7_UNKNOWN,	// ...",
	TXTCL_LANGTABLE7_ORGA,		// Die",
	TXTCL_LANGTABLE7_SIRRUSH,	// Kssksss",
	TXTCL_LANGTABLE7_AZCATL,	// Tkk-tk-tk",
	TXTCL_LANGTABLE7_LEPORI 	// Zboing",
# endif  // ARINDAL
	},
		// log yell
	{
	TXTCL_LANGTABLE8_GIBBERISH,	// shrieks",
	TXTCL_LANGTABLE8_HALFLING,	// shouts",	// shrieks? squeals?
	TXTCL_LANGTABLE8_SYLVAN,	// calls",
	TXTCL_LANGTABLE8_PEOPLE,	// roars",
	TXTCL_LANGTABLE8_THOOM,		// trumpets",	// honks?
	TXTCL_LANGTABLE8_DWARVEN,	// hollers",
	TXTCL_LANGTABLE8_GHORAK_ZO,	// bellows",
	TXTCL_LANGTABLE8_ANCIENT,	// chants loudly",
	TXTCL_LANGTABLE8_MAGIC,		// utters loudly",
	TXTCL_LANGTABLE8_COMMON,	// yells something",
	TXTCL_LANGTABLE8_CODE,		// gestures wildly",
	TXTCL_LANGTABLE8_LORE,		// incants loudly"
# ifdef ARINDAL
	TXTCL_LANGTABLE8_MONSTER,	// growls loudly",
	TXTCL_LANGTABLE8_UNKNOWN,	// shrieks",
	TXTCL_LANGTABLE8_ORGA,		// grunts loudly",
	TXTCL_LANGTABLE8_SIRRUSH,	// hisses loudly",
	TXTCL_LANGTABLE8_AZCATL,	// rattles",
	TXTCL_LANGTABLE8_LEPORI 	// yelps",
# endif  // ARINDAL
	}
};

#else  // ! USE_WHISPER_YELL_LANGUAGE

const char * const
gVerbTable[] =
{
	TXTCL_LANGTABLE3_GIBBERISH,	// babbles",
	TXTCL_LANGTABLE3_HALFLING,	// squeaks",
	TXTCL_LANGTABLE3_SYLVAN,	// chirps",
	TXTCL_LANGTABLE3_PEOPLE,	// purrs",
	TXTCL_LANGTABLE3_THOOM,		// hums",
	TXTCL_LANGTABLE3_DWARVEN,	// mutters",
	TXTCL_LANGTABLE3_GHORAK_ZO,	// rumbles",
	TXTCL_LANGTABLE3_ANCIENT,	// chants",
	TXTCL_LANGTABLE3_MAGIC,		// utters",
	TXTCL_LANGTABLE3_COMMON,	// says something",
	TXTCL_LANGTABLE3_CODE,		// gestures",
	TXTCL_LANGTABLE3_LORE,		// incants"
# ifdef ARINDAL
	TXTCL_LANGTABLE3_MONSTER,	// growls",
	TXTCL_LANGTABLE3_UNKNOWN,	// sounds",
	TXTCL_LANGTABLE3_ORGA,		// grunts",
	TXTCL_LANGTABLE3_SIRRUSH,	// hisses",
	TXTCL_LANGTABLE3_AZCATL,	// clacks",
	TXTCL_LANGTABLE3_LEPORI 	// nibbles",
# endif  // ARINDAL
};

#endif	// USE_WHISPER_YELL_LANGUAGE

/*
	to-do: improve the thieves' language transcriptions. E.g.:
		Gaia touches her left eyebrow with her right fingertip.
		Joe scratches his shoulder.
*/


/*
**	here's the deal...
**	in theory we could make kMaxStoredFrame something larger than 1.
**	like say 2 or 4...
**	and build the frame information based on the deltas.
**	it's a good idea.
**	but i don't think it's practical.
**
**	with one key frame and one delta frame you're doubling the latency
**	when a packet is lost.
**	granted that's one in 100 typically.
**	but we want to be playable when it's 1 in 10.
**	so every 20 frames we lose a delta and a key/delta combo.
**	that's a 1/4 second lag plus a 1/2 second lag every 5 seconds.
**	any more than 1 delta frame and we're hosed.
**
**	the other major problem with delta frame concept is that it requires the
**	server to do more work. ie compute the deltas.
**	and the server's already badly overstressed.
**		[... or at least, it _was_, back when the above was written. Nowadays
**		our virtual machine @ Rackspace hardly even notices the game's load.]
**
**	in v103 caching was enabled for static pictures when a player holds still.
**	this requires kMaxStoredFrame to be 1.
**	if kMaxStoredFrame is changed to anything other than 1 then
**	CCLFrame::ReadKeyFromSpool() will need to be modified (somehow!).
**	probably by copying the static images from the previous frame
**	to the current one.
**
**	-timmer
*/
const int	kMaxStoredFrame = 1;

static int				gFrameIndex	= 0;
static CCLFrame *		gFrames;

#if CL_DO_NIGHT
static NightInfo		gNightInfo;
#endif  // CL_DO_NIGHT

#ifdef CL_DO_WEATHER
static WeatherInfo		gWeatherInfo;
#endif // CL_DO_WEATHER


// map bubble types to position in the image
const uchar				gBubbleMap[] =
{
	0,		// kBubbleNormal
	1,		// kBubbleWhisper
	2,		// kBubbleYell
	3,		// kBubbleThought
	4,		// kBubbleRealAction
	5,		// kBubbleMonster
	4,		// kBubblePlayerAction - same graphic (column) as real action
	3,		// kBubblePonder - the ponder bubble uses the same graphic
			//	as the thought bubble
	4		// kBubbleNarrate - same graphic (column) as the real action bubble
};

#if defined( USE_OPENGL )

using std::cos;
using std::sin;

	// doing it like this means it is done at compile time,
	// not every time the app initializes
const int kUnitCircleSlices = 24;	// how many slices to cut the circle into
const float kUnitCircleStep = kUnitCircleSlices * 2 * F_PI;
const GLfloat kUnitCircle[ kUnitCircleSlices ][2] =
{
	{ cos(  0 / kUnitCircleStep ), sin(  0 / kUnitCircleStep ) },
	{ cos(  1 / kUnitCircleStep ), sin(  1 / kUnitCircleStep ) },
	{ cos(  2 / kUnitCircleStep ), sin(  2 / kUnitCircleStep ) },
	{ cos(  3 / kUnitCircleStep ), sin(  3 / kUnitCircleStep ) },
	{ cos(  4 / kUnitCircleStep ), sin(  4 / kUnitCircleStep ) },
	{ cos(  5 / kUnitCircleStep ), sin(  5 / kUnitCircleStep ) },
	{ cos(  6 / kUnitCircleStep ), sin(  6 / kUnitCircleStep ) },
	{ cos(  7 / kUnitCircleStep ), sin(  7 / kUnitCircleStep ) },
	{ cos(  8 / kUnitCircleStep ), sin(  8 / kUnitCircleStep ) },
	{ cos(  9 / kUnitCircleStep ), sin(  9 / kUnitCircleStep ) },
	{ cos( 10 / kUnitCircleStep ), sin( 10 / kUnitCircleStep ) },
	{ cos( 11 / kUnitCircleStep ), sin( 11 / kUnitCircleStep ) },
	{ cos( 12 / kUnitCircleStep ), sin( 12 / kUnitCircleStep ) },
	{ cos( 13 / kUnitCircleStep ), sin( 13 / kUnitCircleStep ) },
	{ cos( 14 / kUnitCircleStep ), sin( 14 / kUnitCircleStep ) },
	{ cos( 15 / kUnitCircleStep ), sin( 15 / kUnitCircleStep ) },
	{ cos( 16 / kUnitCircleStep ), sin( 16 / kUnitCircleStep ) },
	{ cos( 17 / kUnitCircleStep ), sin( 17 / kUnitCircleStep ) },
	{ cos( 18 / kUnitCircleStep ), sin( 18 / kUnitCircleStep ) },
	{ cos( 19 / kUnitCircleStep ), sin( 19 / kUnitCircleStep ) },
	{ cos( 20 / kUnitCircleStep ), sin( 20 / kUnitCircleStep ) },
	{ cos( 21 / kUnitCircleStep ), sin( 21 / kUnitCircleStep ) },
	{ cos( 22 / kUnitCircleStep ), sin( 22 / kUnitCircleStep ) },
	{ cos( 23 / kUnitCircleStep ), sin( 23 / kUnitCircleStep ) }
};
#endif  // USE_OPENGL



/*
**	InitFrames()
**
**	Initialize the frame buffer
*/
void
InitFrames()
{
	gFrames = NEW_TAG("Frames") CCLFrame[ kMaxStoredFrame ];
	if ( not gFrames )
		SetErrorCode( memFullErr );
	else
		gFrame = gFrames;
}


/*
**	InitGameState()
**
**	Initialize state and various game data
*/
void
InitGameState()
{
	// clear the descriptor table
	bzero( gDescTable, kDescTableSize * sizeof( *gDescTable ) );
	
	// michel: initialize mobile counter
	gNumMobiles			= 0;
	gStateMode			= kStateSizeHi;
	gStateMaxSize		= 0;
	gStateCurSize		= 0;
	gStateExpectSize	= 0;
	delete[] gStatePtr;
	gStatePtr			= nullptr;
	gFlipAllBubbles		= false;
	gNightInfo.Reset();
	
#ifdef CL_DO_WEATHER
	gWeatherInfo.Reset();
#endif // CL_DO_WEATHER
}


/*
**	CreateCLWindow()
**
**	Create and show the Clan Lord window.
*/
void
CreateCLWindow( const DTSRect * pos )
{
	// create the window
	CLWindow * win = NEW_TAG("CLWindow") CLWindow;
	CheckPointer( win );
	
	// save in a global variable
	gGameWindow = win;
	
	// create window of type document
		// normally we do want get-front-click, but if you use click-toggles
		// mode, then you're too often apt to find yourself running aimlessly.
	DTSError result = win->Init( kWKindDocument /* | kWKindGetFrontClick */ );
	SetErrorCode( result );
	CheckError();
	
	// load the text bubble images
	LoadBubbleImages();
	CheckError();
	
	// init the offscreen buffer image
	win->winOffView.Init( kCLDepth );
	CheckError();
	
	// create the offscreen text image
	win->winOffView.offTextView.InitTV();
	CheckError();
	
	// title and position the window
	char buff[ 64 ];
	int major = gPrefsData.pdClientVersion >> 8;
	int minor = gPrefsData.pdClientVersion & 0xFF;
	
#define	TXTCL_GAME_WINDOW_NAME	TXTCL_APP_NAME
	
	if ( minor )
		snprintf( buff, sizeof buff, TXTCL_GAME_WINDOW_NAME " v%d.%d", major, minor );
	else
		snprintf( buff, sizeof buff, TXTCL_GAME_WINDOW_NAME " v%d", major );
	win->SetTitle( buff );
	win->Move( pos->rectLeft, pos->rectTop );
	
	result = InitLayout();
	if ( noErr != result )
		return;
	
	// create the text field, but only once,
	// so we don't lose the text styles on a resize
	result = win->winTextField.Init( win, &gLayout.layoTextBox, "", 10 );
	SetErrorCode( result );
	CheckError();
	
	// do all the dynamic stuff
	DynamicInitCLWindow();
	CheckError();
	
	// Set the background colors based on the state of "Bright Colors"
	SetBackColors();
	
	gTextColorTable[ kColorCodeTextBlack ].Set( 0x0000, 0x0000, 0x0000 );
	gTextColorTable[ kColorCodeTextWhite ].Set( 0xFFFF, 0xFFFF, 0xFFFF );
	gTextColorTable[ kColorCodeTextRed   ].Set( 0xFFFF, 0x0000, 0x0000 );
	gTextColorTable[ kColorCodeTextBlue  ].Set( 0x0000, 0x0000, 0xC000 );
	
	// Ted has this weird way of playing, where he marks everyone as a friend
	// for some unfathomable reason. Then he unfriends all the people who really
	// ARE his friends. Or something like that.  Anyway, this flag lets him do
	// that without too much trauma.
	gSpecialTedFriendMode = false;
	
	// install carbon event handlers
	result = win->InstallEventHandlers();
	__Check_noErr( result );
	
	// show a special message if the images file is missing
	if ( 0 == gImagesVersion )
		ShowInfoText( _(TXTCL_DATAFILES_MISSING_AUTO) );
	
#ifdef OGL_SHOW_DRAWTIME
	// look for the special development preference, so even GMs don't see the
	// drawtimes unless they specifically ask for it, via:
	//		defaults write com.deltatao.clanlord.client DebugShowDrawTimes -bool YES
	gShowDrawTime = Prefs::GetBool( kShowDrawTimePrefKey );
#endif  // OGL_SHOW_DRAWTIME
}


/*
**	LoadBubbleImages()
**
**	load the bubble images non-flushable
*/
void
LoadBubbleImages()
{
	if ( CLWindow * win = static_cast<CLWindow *>( gGameWindow ) )
		{
		win->winOffView.offSmallBubbleImage  = CachePictureNoFlush( kSmallBubblePict  );
		win->winOffView.offMediumBubbleImage = CachePictureNoFlush( kMediumBubblePict );
		win->winOffView.offLargeBubbleImage  = CachePictureNoFlush( kLargeBubblePict  );
		}
}


/*
**	DynamicInitCLWindow()
**
**	initialize the window given that it may have just changed size
*/
void
DynamicInitCLWindow()
{
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	if ( not win )
		return;
	
	// hide the window
	win->Hide();
	
	// initialize the layout to something reasonable
	if ( noErr != InitLayout() )
		return;
	
	// init and allocate the offscreen buffer image
	win->winOffView.offBufferImage.DisposeBits();
	win->winOffView.offBufferImage.Init( nullptr,
			gLayout.layoWinWidth, gLayout.layoWinHeight, kCLDepth );
	CheckError();
	SetErrorCode( win->winOffView.offBufferImage.AllocateBits() );
	CheckError();
	
	// set the offscreen image
	win->winOffView.Hide();
	win->winOffView.offValid = false;
	win->winOffView.SetImage( &win->winOffView.offBufferImage );
	win->winOffView.Show();
	
#ifdef TRANSLUCENT_HEALTH_BARS
	// allocate the status bar image
	win->winOffView.offStatusBar.DisposeBits();
	win->winOffView.offStatusBar.Init( nullptr,
				gLayout.layoHPBarBox.rectRight
					- gLayout.layoHPBarBox.rectLeft,
				gLayout.layoHPBarBox.rectBottom
					- gLayout.layoHPBarBox.rectTop + 2,
				kCLDepth );
	CheckError();
	SetErrorCode( win->winOffView.offStatusBar.AllocateBits() );
	CheckError();
	
	win->winOffView.offStatusBarView.Init( kCLDepth );
	win->winOffView.offStatusBarView.Hide();
	win->winOffView.offStatusBarView.SetImage( &win->winOffView.offStatusBar );
	win->winOffView.offStatusBarView.Show();
	win->winOffView.statusBarTouch = 0;		// shows the bars
#endif	// TRANSLUCENT_HEALTH_BARS
	
	win->winOffView.offMovieSignBlink = 0;
	win->winOffView.offMovieSignShown = false;
	
	// create the edit text field
	InitSendText( win, &gLayout.layoInputBox );
	CheckError();
	
	// create the static text field
	int top = gLayout.layoTextBox.rectTop;
	int numLines = ( gLayout.layoTextBox.rectBottom - top ) / kLineHeight;
	gNumLines = numLines;
	gLayout.layoTextBox.rectBottom = top + numLines * kLineHeight;
	
	win->winTextField.Hide();
	win->winTextField.Move( gLayout.layoTextBox.rectLeft,
							gLayout.layoTextBox.rectTop );
	win->winTextField.Size( gLayout.layoTextBox.rectRight
								- gLayout.layoTextBox.rectLeft,
							gLayout.layoTextBox.rectBottom
								- gLayout.layoTextBox.rectTop );
	win->winTextField.Show();
	
	// size the window
	win->Size( gLayout.layoWinWidth, gLayout.layoWinHeight );
	
	// show the window
	win->winInRedraw = false;
	win->Show();
	
	// clear potential player names from the command menu
	UpdateCommandsMenu( nullptr );
	
	// reset the macro MOVE command mechanism
	gMovePlayer			= false;
	gMovePlayerQuadrant	= move_Stop;
//	gMoveStopIfBalance	= false;
}


/*
**	ChangeWindowSize()
**
**	react to changed window size
**	(but now that the small window is no longer supported,
**	we only call this when the images file might have changed,
**	meaning the bubble images or the background may be stale)
*/
void
ChangeWindowSize()
{
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	if ( not win )
		return;
	
	LoadBubbleImages();
	DynamicInitCLWindow();
	
#ifdef USE_OPENGL
	if ( ctx )
		{
		setupOpenGLGameWindow( ctx, gUsingOpenGL );
		if ( gOGLNightTexture )
			{
			unbindTexture2D( gOGLNightTexture );
			glDeleteTextures( 1, &gOGLNightTexture );
			gOGLNightTexture = 0;				// zero is not a valid object name
			gOGLNightTexturePriority = -1.0f;	// -1 is not a valid priority
			}
		glDeleteLists( gLightList, 1 );
		gLightList = 0;
		}
	if ( gLightmapGWorld )
		{
		::DisposeGWorld( gLightmapGWorld );
		gLightmapGWorld = nullptr;
		}
	if ( gAllBlackGWorld )
		{
		::DisposeGWorld( gAllBlackGWorld );
		gAllBlackGWorld = nullptr;
		}
#endif	// USE_OPENGL
	
	win->GoToFront();
}


/*
**	RedrawCLWindow()
**
**	Redraw offscreen and onscreen because the draw state has changed.
*/
void
RedrawCLWindow()
{
	// paranoia check
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	if ( not win )
		return;
	
	// new stuff for frame-rate averaging:
	const int kNumTimeSamples = 10;				// size of sample history buffer
	
	static int totalTime;						// running total of N most recent frames
	static int sampleCount;						// how many valid samples do we have?
	static int sampleIndex;						// pointer to oldest sample
	static int samples[ kNumTimeSamples ];		// the samples
	static int numSlowFrames, numFastFrames;	// count of out-of-norm samples
	static int oldFastLimit, oldSlowLimit;		// watch that our globs didn't change
	
	// if limits changed (dialog?) -- reset the statistics
	if ( oldFastLimit != gFastDrawLimit || oldSlowLimit != gSlowDrawLimit )
		{
		oldFastLimit = gFastDrawLimit;
		oldSlowLimit = gSlowDrawLimit;
		
		totalTime = 0;
		sampleCount = sampleIndex = 0;
		numSlowFrames = numFastFrames = 0;
		
		bzero( samples, sizeof samples );
		}
	
	// if using opengl, we need to make sure nothing needs
	// to be changed for this frame
#ifdef USE_OPENGL
	setOGLState();
#endif	// USE_OPENGL
	
	// start the draw time timer
	DTSTimer timer;
	timer.StartTimer();
	
	// draw offscreen and on
	win->winOffView.offValid = true;
	win->winOffView.Draw();
	win->winInRedraw = true;
	win->Draw();
	win->winInRedraw = false;
	
//	RedrawPlayersWindow();
	
	// push the animations to the next frame
	UpdateAnims();
	
	// stop the timer
	ulong time = timer.StopTimer() / 1000;	// millisecs
	
	// update total time: add in new, subtract oldest
	totalTime = totalTime + time - samples[ sampleIndex ];
	
	// maybe bump up framespeed counters for this frame
	if ( time > (uint) gSlowDrawLimit )
		++numSlowFrames;
	else
	if ( time < (uint) gFastDrawLimit )
		++numFastFrames;
	
	// and maybe bump them down, for the stalest sample, if we have one
	if ( kNumTimeSamples == sampleCount )
		{
		if ( samples[ sampleIndex ] > gSlowDrawLimit )
			--numSlowFrames;
		else
		if ( samples[ sampleIndex ] < gFastDrawLimit )
			--numFastFrames;
		}
	
	// accumulate this time sample
	if ( sampleCount < kNumTimeSamples )
		++sampleCount;
	
	// replace oldest sample
	samples[ sampleIndex++ ] = time;
	if ( kNumTimeSamples == sampleIndex )
		sampleIndex = 0;
	
#ifdef USE_OPENGL
	// should this be moved to where gFastBlendMode is set (so it's all in one place)?
	int dynamicLoadAdjustment = 1;		// "good"
	if ( time >= (uint) gSlowDrawLimit )
		dynamicLoadAdjustment = -10;	// "bad"
			// alternately, make a function of what is actually being drawn
			// but that can be saved for some future refinement
	else
	if ( time >= (uint) gFastDrawLimit )		// "marginal"
		dynamicLoadAdjustment = 0;
	
	gLightmapRadiusLimit += dynamicLoadAdjustment;
	if ( gLightmapRadiusLimit > kLightmapMaximumRadius )
		gLightmapRadiusLimit = kLightmapMaximumRadius;
	if ( gLightmapRadiusLimit < 0 )
		gLightmapRadiusLimit = 0;
#endif	// USE_OPENGL
	
#if 0
	static int oldradius = -1;
	if ( oldradius != gLightmapRadiusLimit )
		{
		ShowMessage( "gLightmapRadiusLimit = %d", gLightmapRadiusLimit );
		oldradius = gLightmapRadiusLimit;
		}
#endif	// 0
	
	if ( gPrefsData.pdShowDrawTime )
		{
		const char * note = "good";
		if ( time >= (uint) gSlowDrawLimit )
			note = "bad";
		else
		if ( time >= (uint) gFastDrawLimit )
			note = "marginal";
#if 0
		{
		char buff[ 256 ];
		snprintf( buff, sizeof buff,
			"\r" BULLET " Avg draw time: %d msec (f:%d, s:%d). This is %s.",
			totalTime / sampleCount,
			numFastFrames,
			numSlowFrames,
			note );
		ShowInfoText( buff );
		}
#else  // 1
		ShowMessage( "Avg draw time: %d msec (f:%d, s:%d). This is %s.",
			totalTime / sampleCount,
			numFastFrames,
			numSlowFrames,
			note );
#endif  // 1
		}
	
#ifdef OGL_SHOW_DRAWTIME
	gOGLDrawTime = totalTime / sampleCount;
#endif	// OGL_SHOW_DRAWTIME
	
	// if all frames were fast, use good blending...
	if ( kNumTimeSamples == numFastFrames )
		gFastBlendMode = false;
	
	// else if at least some were slow, use bad blending
	else
	if ( numSlowFrames >= kNumTimeSamples / 2 )
		gFastBlendMode = true;
	
	// otherwise, it's a mixed bag; some slow, some fast, no extremes...
	// we'll just leave blend mode alone
	
// alternate strategies:
		// downshift if avg time is long
//	gFastBlendMode = ( (totalTime / sampleCount) >= 245 );
		// downshift if this frame was long
//	gFastBlendMode = ( time >= 245 );
}


/*
**	CLWindow::DoDraw()
**
**	Copy offscreen to on.
*/
void
CLWindow::DoDraw()
{
	// if we are not in a redraw
	// (i.e. if this was called from an updateEvt or kEventWindowDrawContent)
	// then draw everything but the text field and the input field
	// (which will draw themselves)
	DTSRect box;
	if ( not winInRedraw )
		{
		DTSRegion mask;
		DTSRegion rgn;
		
		// mask out the text fields
		DTSRect viewBounds;
		GetBounds( &viewBounds );
		mask.Set( &viewBounds );
		rgn.Set( &gLayout.layoTextBox );
		mask.Subtract( &rgn );
		box = gLayout.layoInputBox;
		box.Inset( kGameInputMargin, kGameInputMargin );
		rgn.Set( &box );
		mask.Subtract( &rgn );
		
#ifdef USE_OPENGL
		// let the GL drawable take care of itself too
		if ( gUsingOpenGL )
			{
			rgn.Set( &gLayout.layoFieldBox );
			mask.Subtract( &rgn );
			}
#endif	// USE_OPENGL
		
		// copy the offscreen buffer onscreen using the mask
		SetMask( &mask );
		Draw( &winOffView.offBufferImage, kImageCopyFast, &viewBounds, &viewBounds );
		SetMask( &viewBounds );
		
		// this will redraw the text fields
		// in case we're in some sort of an update event
		DTSWindow::DoDraw();
		}
	else
		{
		// we _are_ in a redraw (having received a fresh server packet):
		// draw only the hand-objects, health boxes, and playfield.
		// we assume the hand-objects and meter bars are clumped together
		box = gLayout.layoLeftObjectBox;
		box.Union( &gLayout.layoRightObjectBox );
		box.Union( &gLayout.layoHPBarBox );
		box.Union( &gLayout.layoSPBarBox );
		box.Union( &gLayout.layoAPBarBox );
		Draw( &winOffView.offBufferImage, kImageCopyFast, &box, &box );
		
		// draw the field
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
			aglSwapBuffers( ctx );
			
			
			// on general principle, must call this at least once per frame
			ShowOpenGLErrors();
			
# ifdef OGL_USE_UPDATE_OVERRIDE
			EnableScreenUpdates();
# endif	// OGL_USE_UPDATE_OVERRIDE
# if defined(USE_OPENGL) && defined(DEBUG_VERSION) && defined(OGL_SHOW_IMAGE_ID)
			// for finding problem images when using light casters
			ShowMessage( "swap buffer" );
# endif	// USE_OPENGL && DEBUG_VERSION && OGL_SHOW_IMAGE_ID
			}
		else
#endif	// USE_OPENGL
				// blit the offview into the playfield box
			Draw( &winOffView.offBufferImage, kImageCopyFast,
				&gLayout.layoFieldBox, &gLayout.layoFieldBox );
		}
}


/*
**	CLWindow::Activate()
**
**	Activate the menus & text input field
*/
void
CLWindow::Activate()
{
	DTSMenu * menu = gDTSMenu;
	menu->SetFlags( mUndo,       kMenuNormal  );
	menu->SetFlags( mCut,        kMenuNormal  );
	menu->SetFlags( mCopy,       kMenuNormal  );
	menu->SetFlags( mPaste,      kMenuNormal  );
	menu->SetFlags( mClear,      kMenuNormal  );
	menu->SetFlags( mSelectAll,  kMenuNormal  );
	menu->SetFlags( mGameWindow, kMenuChecked );
	gSendText.Activate();
	DTSWindow::Activate();
}


/*
**	CLWindow::Deactivate()
**
**	Deactivate the menus & text input field
*/
void
CLWindow::Deactivate()
{
	DTSMenu * menu = gDTSMenu;
	menu->SetFlags( mUndo,       kMenuDisabled );
	menu->SetFlags( mCut,        kMenuDisabled );
	menu->SetFlags( mCopy,       kMenuDisabled );
	menu->SetFlags( mPaste,      kMenuDisabled );
	menu->SetFlags( mClear,      kMenuDisabled );
	menu->SetFlags( mSelectAll,  kMenuDisabled );
	menu->SetFlags( mGameWindow, kMenuNormal   );
	gSendText.Deactivate();
	DTSWindow::Deactivate();
	
	gClickState = 0;
}


/*
**	SetPlayerMenuItem()
**
**	set a player-related entry in the Command menu
**	for the currently-selected player
*/
static void
SetPlayerMenuItem( int mCmd, const char * cmd, const char * name )
{
	char buff[ 256 ];
	snprintf( buff, sizeof buff, "%s %s", cmd, name );
	gDTSMenu->SetText( mCmd, buff );
}


/*
**	UpdateCommandsMenu()
**
**	add selected player name to items in Command menu
**	(or self, if none selected)
*/
void
UpdateCommandsMenu( const char * newPlyrName )
{
	DTSMenu * menu = gDTSMenu;
	if ( not menu )
		return;
	
#if 0
// uncomment this part if you want your own name in the menu
// when no players are selected

	if ( not newPlyrName || 0 == newPlyrName[0] )
		{
		// no player selected, try to use 'me'
		if ( gThisPlayer )
			newPlyrName = gThisPlayer->descName;
		}
#endif  // 0
	
	if ( not newPlyrName || 0 == newPlyrName[0] )
		{
		// no usable name -- let the server handle it
		menu->SetText( mInfo,		"Info" );
		menu->SetText( mPull,		"Pull" );
		menu->SetText( mPush,		"Push" );
		menu->SetText( mKarma,		"Karma" );
		menu->SetText( mThank,		"Thank" );
		menu->SetText( mCurse,		"Curse" );
		menu->SetText( mShare,		"Spirit Link" );
		menu->SetText( mUnshare,	"Un-Link" );
		
		menu->SetText( mBefriend,	"Label" );
		menu->SetText( mBlock,		"Block" );
		menu->SetText( mIgnore,		"Ignore" );
		menu->SetFlags( mBefriend,	kMenuDisabled );
		menu->SetFlags( mBlock,		kMenuDisabled );
		menu->SetFlags( mIgnore,	kMenuDisabled );
		}
	else
		{
		// have a player
		SetPlayerMenuItem( mInfo,	"Info", newPlyrName );
		SetPlayerMenuItem( mPull,	"Pull", newPlyrName );
		SetPlayerMenuItem( mPush,	"Push", newPlyrName );
		SetPlayerMenuItem( mKarma,	"Karma", newPlyrName );
		SetPlayerMenuItem( mThank,	"Thank", newPlyrName );
		SetPlayerMenuItem( mCurse,	"Curse", newPlyrName );
		SetPlayerMenuItem( mShare,	"Spirit Link", newPlyrName );
		SetPlayerMenuItem( mUnshare, "Un-Link", newPlyrName );
		
		SetPlayerMenuItem( mBefriend, "Label", newPlyrName );
		int friends = GetPlayerIsFriend( gSelectedPlayerName );
		switch ( friends )
			{
			case kFriendLabel1:
			case kFriendLabel2:
			case kFriendLabel3:
			case kFriendLabel4:
			case kFriendLabel5:
				menu->CheckRange( mNoLabel, mPurpleLabel,
					mRedLabel + friends - kFriendLabel1 );
				break;
			
			default:
				menu->CheckRange( mNoLabel, mPurpleLabel, mNoLabel );
				break;
			}
		
		SetPlayerMenuItem( mBlock,
				(kFriendBlock == friends)  ? "Unblock"  : "Block",	newPlyrName );
		SetPlayerMenuItem( mIgnore,
				(kFriendIgnore == friends) ? "Unignore" : "Ignore",	newPlyrName );
		
		menu->SetFlags( mBefriend,	kMenuNormal );
		menu->SetFlags( mBlock,		kMenuNormal );
		menu->SetFlags( mIgnore,	kMenuNormal );
		}
}


/*
**	MovePlayerToward()
**	script/macro movement helper
** If the mouse is not clicked, we move the guy toward somewhere
*/
bool
MovePlayerToward( int quadrant, int moveSpeed, bool /* stopIfBalance */ )
{
	// real mouse-downs take precedence over scripted motion commands
	if ( gClickState )
		return false;
	
	int speed = ( gLayout.layoFieldBox.rectRight
				- gLayout.layoFieldBox.rectLeft ) / 2;
	
	if ( move_speed_Walk == moveSpeed )
		speed /= 4;		// sorta walking speed
	else
	if ( move_speed_Stop == moveSpeed )
		speed = 4;		// a speed big enough for turning!
	
	// .707 = approx 1/sqrt(2) -- so that diagonal motion is
	// roughly same speed (in pixels/second) as non-diagonal
	int diagSpeed = (speed * 707) / 1000;
	
	gMovePlayer = true;
	
/*
	// No more coasting, but you can do things like turn now -  Jeff
	
	// toggle move quadrants
	if ( gMovePlayer && gMovePlayerQuadrant == quadrant )
		gMovePlayer = false;
	else
		gMovePlayer	= quadrant >= move_Stop && quadrant <= move_SouthEast;
*/
	
//	gMoveStopIfBalance = stopIfBalance;  // Uh what is this for? - Jeff
	// I think it _may_ have been meant to help support a "flight or fight" mode,
	// where you run away from a monster until you recover enough balance to
	// turn around and swing at it. But if so, it was never finished; and the
	// 'stopIfBalance' parameter should really be extirpated.
	
	if ( gMovePlayer )
		{
		gMovePlayerQuadrant	= quadrant;
		switch ( quadrant )
			{
			case move_Stop:
				gMovePlayer	= false;
				gClickLoc.Set( 0, 0 );	// send a zero packet to stop walking
				break;
			case move_East:
				gClickLoc.Set( +speed, 0 );
				break;
			case move_NorthEast:
				gClickLoc.Set( +diagSpeed, -diagSpeed );
				break;
			case move_North:
				gClickLoc.Set( 0, -speed );
				break;
			case move_NorthWest:
				gClickLoc.Set( -diagSpeed, -diagSpeed );
				break;
			case move_West:
				gClickLoc.Set( -speed, 0 );
				break;
			case move_SouthWest:
				gClickLoc.Set( -diagSpeed, +diagSpeed );
				break;
			case move_South:
				gClickLoc.Set( 0, +speed );
				break;
			case move_SouthEast:
				gClickLoc.Set( +diagSpeed, +diagSpeed );
				break;
			}
		}
	return gMovePlayer;
}


/*
**	CLWindow::Click()
**
**	handle a click in the window
*/
bool
CLWindow::Click( const DTSPoint& loc, ulong time, uint modifiers )
{
	int newState = 0;
	
	// decide which of the three boxes the click occured in
	int flag = 0;
	if ( loc.InRect( &gLayout.layoFieldBox ) )
		{
		
		// Handle macro clicks, which might totally override other clicks.
		if ( DoMacroClick( &loc, modifiers ) )
			return true;
		
		if ( modifiers & (kKeyModMenu | kKeyModOption | kKeyModControl) )
			{
			DescTable * desc = LocateMobileByPoint( &loc, kDescPlayer );
			newState = gClickState;
			flag = gClickFlag;
			
#ifdef AUTO_HIDENAME
			if ( desc )
				{
				desc->descSeenFrame = gFrameCounter + kNameDelay_HoverShow;
				desc->descNameVisible = kName_Visible;
				}
#endif
			// test modifier-key state
			const uint modsMask = kKeyModMenu
								| kKeyModOption
								| kKeyModControl
								| kKeyModShift;
			switch ( modifiers & modsMask )
				{
					// command-click: just select
				case kKeyModMenu :
					SelectPlayer( desc ? desc->descName : nullptr );
					break;
				
					// option-click: insert name into text box
				case kKeyModOption :
					if ( desc )
						InsertName( desc->descName );
					break;
				
					// control-click: advance label color
				case kKeyModControl :
					if ( desc )
						{
							// befriend self to activate TedMode(tm)
						if ( desc == gThisPlayer )
							gSpecialTedFriendMode = true;
						
						int label = GetPlayerIsFriend( desc );
						label = GetNextFriendLabel( label );
						
						SetPlayerIsFriend( desc, label );
						}
					break;
				
					// shift-control-click: advance label color the other way
				case kKeyModControl | kKeyModShift :
					if ( desc )
						{
						int isAFriend = GetPlayerIsFriend( desc );
						
						if ( kFriendBlock == isAFriend )
							isAFriend = kFriendIgnore;
						else
						if ( kFriendIgnore == isAFriend )
							isAFriend = kFriendNone;
						else
							isAFriend = kFriendBlock;
						
						SetPlayerIsFriend( desc, isAFriend );
						}
					break;
				
				default:
					break;
				}
			}
		else
			{
			// unadorned click: movement
			flag = kPIMDownField;
			gLastClickGame = true;
			gClickLoc.Set( loc.ptH - gFieldCenter.ptH, loc.ptV - gFieldCenter.ptV );
			
			// implement the "toggle" part of click-toggles mode
			if ( kMoveClickToggle == gPrefsData.pdMoveControls )
				newState = not gClickState;
			}
		}
	else
	// click outside playfield
	if ( loc.InRect( &gLayout.layoLeftObjectBox ) )
		ClickSlotObject( kItemSlotLeftHand );
	else
	if ( loc.InRect( &gLayout.layoRightObjectBox ) )
		ClickSlotObject( kItemSlotRightHand );
	
	gClickFlag = flag;
	gClickState = newState;
	
//	if ( gClickState ) - Jeff weird mouse control when clicking after a
						// movement macro using click and hold
		gMovePlayer	= false;		// cancel keyboard movement!
	
	// if the click was not in one of our rectangles then
	// return false telling the system to keep looking for a view
	// to handle the click (e.g. the input text box)
	if ( not flag )
		return DTSWindow::Click( loc, time, modifiers );
	
	// otherwise we handled it
	return true;
}


/*
**	CLWindow::KeyStroke()
**
**	handle a keystroke
*/
bool
CLWindow::KeyStroke( int ch, uint modifiers )
{
	bool bResult = false;
	
	// is this a movie-playback speed adjustment?
	if ( CCLMovie::IsReading() && (modifiers & kKeyModMenu) )
		{
		int delta = 0;
		switch ( ch )
			{
			// cmd-right: speed up playback
			case kRightArrowKey:
				delta = +1;
				break;
			
			// cmd-left: slow down
			case kLeftArrowKey:
				delta = -1;
				break;
			}
		
		if ( delta != 0 )
			{
			CCLMovie::SetReadSpeed( CCLMovie::GetReadSpeed() + delta );
			winOffView.offMovieSignBlink = 0;
			winOffView.offMovieSignShown = false;
			bResult = true;
			}
		}
	
	// otherwise, forward the keystroke to the input box
	if ( not bResult )
		bResult = SendTextKeyStroke( ch, modifiers );
	
	// deobscure the cursor, if it's over the playfield
	DTSPoint mouseLoc;
	GetMouseLoc( &mouseLoc );
	if ( mouseLoc.InRect( &gLayout.layoFieldBox ) )
		DTSShowMouse();
	
	return bResult;
}


/*
**	CLWindow::Idle()
**
**	flash the text field cursor
*/
void
CLWindow::Idle()
{
	// if we're not actually playing...
	// and if we don't need to blink the caret...
	// we can sleep a long, long time
	if ( not gInBack )
		{
		int sleep = 6;
		if ( not gPlayingGame )
			{
			int start, stop;
			gSendText.GetSelect( &start, &stop );
			if ( start == stop )
				sleep = ::GetCaretTime();
			else
				sleep = 60;		// 1 sec. Was 0x0FFFFFF, but if we sleep too long
								// then the CharMgr dialog locks up.
			}
		else
		if ( CCLMovie::GetReadSpeed() == CCLMovie::speed_Turbo )
			{
			// for "turbo" movie playback speed, we need super-short naps
			sleep = 1;
			}
		
		gDTSApp->SetSleepTime( sleep );
		}
	
	gSendText.Idle();
	winTextField.Idle();  // JEB Update cursor if it is over a link
	
	// keep cursor-pointer image up to date
	DTSPoint loc;
	GetMouseLoc( &loc );
	if ( loc.InRect( &gLayout.layoInputBox ) )
		gCursor = iBeamCursor;
	else
	if ( loc.InRect( &gLayout.layoFieldBox ) )
		{
		if ( kMoveClickHold == gPrefsData.pdMoveControls
		&&	 IsButtonDown()
		&&	 gPlayingGame
		&&	 not IsKeyDown( kOptionModKey )
		&&	 not IsKeyDown( kControlModKey )
		&&	 not IsKeyDown( kMenuModKey ) )
			{
			// slightly different than the click-toggles one
			gCursor = 129;
			}
		
#ifdef AUTO_HIDENAME
		// hovering over an auto-hidden player name reveals it temporarily
		if ( true )
			{
			DescTable * desc = LocateMobileByPoint( &loc, kDescUnknown );
			if ( not gHoveringOnPlayer || gHoveringOnPlayer != desc )
				{
				gHoveringOnPlayer	= desc;
				gHoveringSince		= gFrameCounter;
				}
			else
				{
				int hoverDelay = (desc == gThisPlayer)
								? kNameDelay_HoverSelf : kNameDelay_Hover;
				if ( gFrameCounter - gHoveringSince >= hoverDelay )
					{
					int newWhen = gFrameCounter + kNameDelay_HoverShow;
					// set it only if it is useful
					if ( newWhen > desc->descSeenFrame )
						desc->descSeenFrame = newWhen;
					desc->descNameVisible	= kName_Visible;
					}
				}
			}
#endif	// AUTO_HIDENAME
		}
}


/*
**	CLWindow::DoCmd()
**
**	all menu commands go to the send text field
*/
bool
CLWindow::DoCmd( long menuCmd )
{
	return SendTextCmd( menuCmd );
}


// carbon-event handling

/*
**	CLWindow::InstallEventHandlers()
**	add a handler for those carbon-events we want to look at
*/
DTSError
CLWindow::InstallEventHandlers()
{
	OSStatus result = InitTarget( GetWindowEventTarget( DTS2Mac(this) ) );
	if ( noErr != result )
		return result;
	
	const EventTypeSpec evts[] =
		{
			{ kEventClassMouse,		kEventMouseWheelMoved	},
			{ kEventClassMouse,		kEventMouseDown			},
#ifdef USE_OPENGL
			{ kEventClassWindow,	kEventWindowBoundsChanged },
#endif
			{ kEventClassWindow,	kEventWindowDrawContent }
		};
	
	result = AddEventTypes( GetEventTypeCount( evts ), evts );
	
	return result;
}


/*
**	CLWindow::HandleEvent()
**	dispatch an event to specific class methods
*/
OSStatus
CLWindow::HandleEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	UInt32 kind = evt.Kind();
	
	switch ( evt.Class() )
		{
		case kEventClassMouse:
			switch ( kind )
				{
				case kEventMouseDown:
					result = DoMouseDownEvent( evt );
					break;
				
				case kEventMouseWheelMoved:
					result = DoWheelEvent( evt );
					break;
				}
			break;
		
		case kEventClassWindow:
			result = DoWindowEvent( evt );
			break;
		}
	
	return result;
}


/*
**	EventModifiersToDTS()
**	synthesize DTS-style event modifier flags from a carbon event
*/
static uint
EventModifiersToDTS( const CarbonEvent& evt )
{
	UInt32 macMods = 0;
	evt.GetParameter( kEventParamKeyModifiers, typeUInt32, sizeof macMods, &macMods );
	return Mac2DTSModifiers( macMods );
}


/*
**	CLWindow::DoMouseDownEvent()
**	handle clicks from alternate buttons
**
**	instead of redoing ALL the click stuff to be based off of here, we'll only
**	deal with clicks from the non-primary button. For the main button, simply
**	return a not-handled result, and let the mDown event arrive conventionally,
**	through WNE, as before.
*/
OSStatus
CLWindow::DoMouseDownEvent( const CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	// we don't care about any clicks if we're not playing, or not connected
	if ( not gPlayingGame
	||	 not gNetworking
	||	 CCLMovie::IsReading() )
		{
		return result;
		}
	
	// get the relevant data from the event
	UInt16 button = 0;
	OSStatus err;
	//if ( noErr == err )
		{
		// extract the button index
		err = evt.GetParameter( kEventParamMouseButton,
					typeMouseButton, sizeof button, &button );
		__Check_noErr( err );
		}
	
	// ignore clicks from the principal button
	if ( 1 == button )
		return result;
	
	// grab the button chord while we're at it
	UInt32 chord;
	if ( noErr == err )
		{
		err = evt.GetParameter( kEventParamMouseChord,
					typeUInt32, sizeof chord, &chord );
		
		// we'll probably get an error on OS 9 here; if so, don't make a fuss
		if ( eventParameterNotFoundErr == err )
			{
			chord = 1U << (button - 1);	// make it look like a uni-button event
			err = noErr;
			}
		__Check_noErr( err );
		}
	
	// get click location (relative to window's structure region)
	DTSPoint loc;
	if ( noErr == err )
		{
		err = evt.GetParameter( kEventParamWindowMouseLocation,
				typeQDPoint, sizeof(Point), DTS2Mac(&loc) );
		__Check_noErr( err );
		}
	
	// see if it's in-bounds
	if ( noErr == err )
		{
		// convert click location to content-region-relative coords
//		DTSPoint winPos;
//		GetPosition( &winPos );
//		loc.Offset( -winPos.ptH, -winPos.ptV );
		Rect strucWidths;
		__Verify_noErr( GetWindowStructureWidths( DTS2Mac(this), &strucWidths ) );
		loc.Offset( -strucWidths.left, -strucWidths.top );
		
		// we only care about clicks in the play field
		if ( not loc.InRect( &gLayout.layoFieldBox ) )
			return result;
		
		// send a 'clickN' event
		uint modifiers = EventModifiersToDTS( evt );
		
		// was the click handled by some macro?
		bool macroDone = DoMacroClick( &loc, modifiers, button, chord );
		
		// Now, there's one oddball case to consider:
		// The event system on X magically transmutes right-clicks (ie button
		// #2) into control-clicks (of button #1), but only when the event is
		// fetched via WaitNextEvent(). However, if the event is received via a
		// carbon-event mousedown handler (such as this very stretch of code
		// right here: what a coincidence!), we get told the real facts, not
		// that little white lie.
		//
		// What that means is, if we were to return noErr here after receiving a
		// #2 click, the control-conversion wouldn't happen, since the click
		// wouldn't ever propagate through to WaitNextEvent().
		//
		// The problem is, players have lots of existing macros mapped to
		// ctrl-click which they rely on being able to trigger with their #2
		// button, so we have to make sure that we don't break them.

		// So... if the button number was 3 or higher (we already rejected #1,
		// above), or if a macro has _specifically_ handled this #2-click, we'll
		// return noErr, so that the event is considered fully handled and won't
		// come back to haunt us a second time, via WNE.
		
		if ( button != 2  ||  macroDone )
			result = noErr;
		
		// otherwise, viz., if button == 2 and no macro claimed it,
		// we'll return eventNotHandledErr so that the event will reappear
		// at WNE time, all nicely mutated into a control-click.
		// Rah rah rah!
		
		// The only fly remaining in the ointment that I can see is this: if the
		// player has a macro specifically bound to click#2, AND if that macro
		// has its $no_override attribute set, AND they also have a macro bound
		// to ctrl-click, THEN DoMacroClick() above will run the first macro and
		// return 'false'; we'll correctly return the not-handled code; the
		// click will get turned into a control-click in WNE; and their other
		// macro will _also_ run. Two macros for the price of one, not bad!
		//
		// But hey, we can just call that a feature, right? :>
		}
	
	return err ? err : result;
}


/*
**	CLWindow::DoWheelEvent()
**	deal with scroll wheels
*/
OSStatus
CLWindow::DoWheelEvent( const CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	// we don't care about wheel events if we're not playing, or not connected
	if ( not gPlayingGame
	||	 not gNetworking
	||	 CCLMovie::IsReading() )
		{
		return result;
		}
	
	DTSRect viewBounds;
	gSendText.GetBounds( &viewBounds );
	
	EventMouseWheelAxis axis = -1;	// invalid axis
	OSStatus err;
	// if ( noErr == err )
		{
		err = evt.GetParameter( kEventParamMouseWheelAxis,
				typeMouseWheelAxis, sizeof axis, &axis );
		__Check_noErr( err );
		}
	
	SInt32 delta = 0;
	if ( noErr == err )
		{
		err = evt.GetParameter( kEventParamMouseWheelDelta,
					typeSInt32, sizeof delta, &delta );
		__Check_noErr( err );
		}
	
	DTSPoint loc;
	if ( noErr == err )
		{
		err = evt.GetParameter( kEventParamWindowMouseLocation, typeQDPoint,
				sizeof(Point), DTS2Mac(&loc) );
		__Check_noErr( err );
		}
	
	// convert to local coordinates
//	DTSPoint winPos;
//	GetPosition( &winPos );
//	loc.Offset( -winPos.ptH, -winPos.ptV );
	Rect wWidths;
	__Verify_noErr( GetWindowStructureWidths( DTS2Mac(this), &wWidths ) );
	loc.Offset( - wWidths.left, - wWidths.top );
	
	// where was it?
	if ( noErr == err
	&&	 loc.InRect( &viewBounds ) )
		{
		// over text field - perform back|forward in input
		SendTextKeyStroke( (delta > 0) ? kUpArrowKey : kDownArrowKey, kKeyModMenu );
		result = noErr;
		}
	else
		{
		// over game window but not text field - activate macro
		// (should this be restricted to just the play field instead?)
		DTSRect winBounds;
		GetBounds( &winBounds );
		if ( loc.InRect( &winBounds ) )
			{
#ifdef DEBUG_VERSION
//			ShowMessage( "scroll evt: axis %u, delta %d", axis, (int) delta );
#endif
			// translate keyboard modifiers from Mac to DTS
			uint modifiers = EventModifiersToDTS( evt );
			int dir = 0;
			if ( kEventMouseWheelAxisY == axis )
				dir = (delta > 0) ? kKeyWheelUp : kKeyWheelDown;
			else
			if ( kEventMouseWheelAxisX == axis )
				dir = (delta > 0) ? kKeyWheelLeft : kKeyWheelRight;
			
			DoMacroKey( &dir, modifiers );
			result = noErr;
			}
		}
	
	return result;
}


/*
**	CLWindow::DoWindowEvent()
**	handle kEventClassWindow carbon events
*/
OSStatus
CLWindow::DoWindowEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	switch ( evt.Kind() )
		{
		case kEventWindowDrawContent:
			{
			bool savedRedraw = winInRedraw;
			winInRedraw = false;
			DoDraw();
			winInRedraw = savedRedraw;
			result = noErr;
			}
			break;
		
#ifdef USE_OPENGL
		case kEventWindowBoundsChanged:
			{
			if ( gUsingOpenGL && ctx )
				aglUpdateContext( ctx );
			}
			break;
#endif
		}
	
	return result;
}


/*
**	CLOffView::DoDraw()
**
**	Redraw the offscreen buffer.
*/
void
CLOffView::DoDraw()
{
	// check if the offscreen has been drawn at least once.
	if ( not offValid )
		DrawFirstTime();
	else
		DrawValid();
}


/*
**	CLOffView::DrawFirstTime()
*/
void
CLOffView::DrawFirstTime()
{
	// erase the field
	DTSRect viewBounds;
	GetBounds( &viewBounds );
	Erase( &viewBounds );
	
	// draw the background picture
	ImageCache * ic = CachePicture( gLayout.layoPictID );
	if ( ic )
		{
		DTSRect icBounds;
		ic->icImage.cliImage.GetBounds( &icBounds );
		Draw( &ic->icImage.cliImage, kImageCopyFast, &icBounds, &icBounds );
		}
	else
		{
#if 0
// this code temporarily disabled until the images are finalized,
// and the project is updated to include them
		
		// bg image is missing from images file...
		// assume it's because this client is freshly downloaded from web;
		//	and images & sounds are missing;
		//	client hasn't yet gotten around to auto-downloading them;
		// so use the baked-in default images instead.
		
		// Note, those images are 'PICT' resources whose IDs are
		// equal to 256 + the corresponding layoPictID
		// (the 256 offset is so we don't impinge on the historical
		// system resource ID space, below #128)
		DTSPicture defaultPict;
		DTSError result = defaultPict.Load( 256 + gLayout.layoPictID );
		if ( noErr == result )
			{
			DTSRect pictBounds;
			defaultPict.GetBounds( &pictBounds );
			Draw( &defaultPict, &pictBounds );
			}
#endif	// 0
		}
}


#ifdef OGL_SHOW_DRAWTIME
/*
**	CLOffView::ShowDrawTime()
**	display some debugging statistics
*/
void
CLOffView::ShowDrawTime()
{
	if ( gUsingOpenGL )
		{
		disableBlending();
		disableAlphaTest();		// experiment with this one...
		disableTexturing();
		glColor3us( 0xFFFF, 0, 0 );
		}
	else
		{
		SetForeColor( &DTSColor::red );
		
		// use 9 point geneva
		SetFont( "Geneva" );
		SetFontSize( 9 );
		SetFontStyle( normal );
		}
	
# if 1
	//
	// count up the cache population
	//
	int cachedTextures = 0;
	if ( gUsingOpenGL )
		{
		for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
			{
			if ( walk->IsImage() )
				{
#  ifdef DEBUG_VERSION	// USE_OPENGL already defined
				ImageCache * ic = dynamic_cast<ImageCache *>( walk );
				if ( not ic )
					ShowMessage( "fake ImageCache * detected" );
				else
#  else
				ImageCache * ic = static_cast<ImageCache *>( walk );
#  endif  // DEBUG_VERSION
				
				if ( ic && ic->textureObject )
					cachedTextures += ic->textureObject->getTextureCount();
				}
			}
		}
# endif  // 1
	
	char buff[ 256 ];
	if ( gUsingOpenGL )
		snprintf( buff, sizeof buff, "%d images, %d textures, %d ms, %s (0x%x)",
			gRootCacheObject->sCachedImageCount, cachedTextures, int(gOGLDrawTime),
			gOGLRendererString, (uint) gOGLRendererID );
	else
		snprintf( buff, sizeof buff, "%d images, %d ms, QuickDraw",
			gRootCacheObject->sCachedImageCount, int(gOGLDrawTime) );
	
	DTSCoord h = gLayout.layoFieldBox.rectLeft + 5;
	DTSCoord v = gLayout.layoFieldBox.rectBottom - 5 - kTextLineHeight * 0;
	
	if ( gUsingOpenGL )
		drawOGLText( h, v, geneva9NormalListBase, buff );
	else
		Draw( buff, h, v, kJustLeft );
	
	if ( not gUsingOpenGL )
		{
		// restore the color/style
		SetForeColor( &DTSColor::black );
		SetFontStyle( normal );
		}
}
#endif  // OGL_SHOW_DRAWTIME


/*
**	CLOffView::DrawValid()
**	redraw the offscreen buffer. The background image's pixels are already in place.
*/
void
CLOffView::DrawValid()
{
#ifdef USE_OPENGL
	if ( gUsingOpenGL )
		{
		if ( gOpenGLEnableEffects )
			fillNightTextureObject();
		else
			gUseLightMap = false;
		
		// clear to gray
		const GLfloat clearColorf = GLfloat(0x8888) / 0xFFFF;
		glClearColor( clearColorf, clearColorf, clearColorf, 0.0 );
//		glClearStencil defaults to 0x0
# ifdef OGL_USE_UPDATE_OVERRIDE
		DisableScreenUpdates();
# endif	// OGL_USE_UPDATE_OVERRIDE
		// harmless if no stencil buffer
		glClear( GL_COLOR_BUFFER_BIT | GL_STENCIL_BUFFER_BIT );
		if ( gOGLManageTexturePriority )
			{
			// if "certain drivers" properly implemented the Least-Recently-
			// Used algorithm, I wouldn't have to do it for them (grumble)
			DecrementTextureObjectPriorities();
			}
		}
	else
#endif	// USE_OPENGL
		{
#if defined( USE_OPENGL) && defined( OGL_QUICKDRAW_LIGHT_EFFECTS )
		// create (yet another) general pref?  or assume everyone?
		fillNightTextureObject();
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
		
		// clip out all but the playfield
		SetMask( &gLayout.layoFieldBox );
		
		// paint areas not drawn by pictures in gray
		const short kBGgrey = 0x8888;
		const DTSColor background( kBGgrey, kBGgrey, kBGgrey );
		SetForeColor( &background );
		Paint( &gLayout.layoFieldBox );
		SetForeColor( &DTSColor::black );
		}
	
	// this removes leftover mobiles, after disconnecting
	if ( not gPlayingGame )
		gNumMobiles = 0;
	
	// draw all of the pictures below the players
	DrawQueuedPictures( +0x0000 );
	
	// sort the mobiles from top to bottom
	SortMobiles();
	
	// draw all the mobile shadows (under the names)
#ifdef USE_OPENGL
	if ( gUsingOpenGL
	&&   gOpenGLEnableEffects
	&&   not gOGLDisableShadowEffects
	&&   gOGLStencilBufferAvailable )
		{
			// four pass multitexturing with stencil (ooooo....)
			// we're going to reduce the alpha test in stages,
			// drawing the darkest areas first
		float shadowLevel = GetShadowLevel() / 100.0f;
		glAlphaFunc( GL_EQUAL, shadowLevel );
		DrawMobileShadows();
		glAlphaFunc( GL_GEQUAL, shadowLevel * 2.0f / 3.0f );
		DrawMobileShadows();
		glAlphaFunc( GL_GEQUAL, shadowLevel / 3.0f );
		DrawMobileShadows();
		glAlphaFunc( GL_NOTEQUAL, 0.0f );	// reset to what it was before
		}
#endif	// USE_OPENGL
	
	DrawMobileShadows();
	
#ifdef USE_OPENGL
	if ( gUsingOpenGL
	&&   gOpenGLEnableEffects
	&&   not gOGLDisableShadowEffects
	&&   gOGLStencilBufferAvailable )
		{
		disableStencilTest();
		}
	
	DrawSelectionHilite();
#endif	// USE_OPENGL
	
	// draw all of the mobile names
	DrawNames();
	
	// draw all of the mobiles
	DrawMobiles();
	
	// draw all of the pictures above the players
	DrawQueuedPictures( +0x7FFF );
	
#ifdef USE_OPENGL
	if ( gUseLightMap )	// gOpenGLEnableEffects is implicit in this
		{
		if ( gUsingOpenGL )
			drawNightTextureObject();
# ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
		else	// gLightmapGWorld is implicit here
			{
// the CopyDeepMask path is twice as slow, but looks much better.  much, much
// better. the qdApplyLightmap path looks awful, but is twice as fast. i'd be
// inclined to stay with the CopyDeepMask, but the whole point of this exercise
// is to support slow computers, and we already have the gl client for folks at
// the other of the spectrum who want "pretty".
//
// update: the client already has support for selecting between fast/ugly and
// slow/quality blitters (gFastBlendMode), so we can just use that.

// next up: figure out some way to get rid of gAllBlackGWorld (because it
// contributes nothing to the end result; it's only there to ensure the first
// half of the blend equation does nothing)
//
// next next up: create a secondary 32 bit color buffer to use in place of the 8
// bit buffer, which will eliminate the 32->8 bit conversion, as well as
// eliminating the nasty banding we are getting.  (if the user has their monitor
// set to 8 or 16 bit color, the OS/video system will do a downsample for us
// anyway.)  the only downside will be that some elements (speech balloons,
// etc.) are drawn after the night shading, and will need 32-bit versions to
// support this new scheme.

			if ( gFastBlendMode )
				{
				// this is much faster now, but we've lost the dither,
				// so it looks like a giant bull's eye
				qdApplyLightmap();
				}
			else
				{
				// the GWorld was not created with the pixPurge flag,
				// so it can't be purged, so LockPixels can't fail (right?)
				::LockPixels( ::GetGWorldPixMap ( gLightmapGWorld ) );
				::LockPixels( ::GetGWorldPixMap ( gAllBlackGWorld ) );
				
				::CopyDeepMask(
					::GetPortBitMapForCopyBits( gAllBlackGWorld ),
					::GetPortBitMapForCopyBits( gLightmapGWorld ),
					DTS2Mac(&offBufferImage),
					DTS2Mac(&gLayout.layoFieldBox),
					DTS2Mac(&gLayout.layoFieldBox),
					DTS2Mac(&gLayout.layoFieldBox),
					srcCopy + ditherCopy	// equivalent to ditherCopy alone,
											// since srcCopy == 0
//					srcCopy					// this makes it look
											// just like qdApplyLightmap()
					nullptr
				);
				
				::UnlockPixels( ::GetGWorldPixMap ( gAllBlackGWorld ) );
				::UnlockPixels( ::GetGWorldPixMap ( gLightmapGWorld ) );
				}
			}
# endif  // OGL_QUICKDRAW_LIGHT_EFFECTS
		}
#endif  // USE_OPENGL
	
#ifdef TRANSLUCENT_HAND_ITEMS
	// draw the floating hand items
	DrawTranslucentHandObjects();
#endif
	
	// draw all of the bubble text
	DrawBubbles();
	
#ifdef TRANSLUCENT_HEALTH_BARS
	// draw the floating health bars (above bubbles)
	DrawTranslucentHealthBars();
#endif
	
	// Draw a sign when recording/playing
	if ( CCLMovie::HasMovie() )
		DrawMovieMode();
	
#ifdef OGL_SHOW_DRAWTIME
	if ( gShowDrawTime )
		ShowDrawTime();
#endif	// OGL_SHOW_DRAWTIME
	
	// one or both of DrawHandObjects() and DrawBar() may be done in a future
	// ogl version, but not now
	
	// reset the clipping mask
	DTSRect viewBounds;
	GetBounds( &viewBounds );
	SetMask( &viewBounds );
	
	// draw the objects held in the hands
	DrawHandObjects();
	
	// draw the hit point bar and spell point bar
	DrawBar( &gLayout.layoHPBarBox, gFrame->mHP,  gFrame->mHPMax );
	DrawBar( &gLayout.layoSPBarBox, gFrame->mSP,  gFrame->mSPMax );
	DrawBar( &gLayout.layoAPBarBox, gFrame->mBalance, gFrame->mBalanceMax );
	
	// let's throw a little progress-indicator needle atop the HP meter
	// during movie playback
	if ( CCLMovie::IsReading() )
		DrawMovieProgress( gLayout.layoHPBarBox );
	
	// if communications have ceased, grey out the playfield
	if ( not gPlayingGame )
		DrawFieldInactive();
	
#if 0	// blit-speed test scaffolding from long ago
	{
	DTSRect dst;
	dst.rectTop    = ( ( gLayout.layoFieldBox.rectBottom + gLayout.layoFieldBox.rectTop  )
		- ( offSmallBubbleImage->icBox.rectBottom - offSmallBubbleImage->icBox.rectTop  ) ) / 2;
	dst.rectLeft   = ( ( gLayout.layoFieldBox.rectRight  + gLayout.layoFieldBox.rectLeft )
		- ( offSmallBubbleImage->icBox.rectRight - offSmallBubbleImage->icBox.rectLeft ) ) / 2;
	dst.rectBottom = dst.rectTop
		+ offSmallBubbleImage->icBox.rectBottom - offSmallBubbleImage->icBox.rectTop;
	dst.rectRight  = dst.rectLeft
		+ offSmallBubbleImage->icBox.rectRight  - offSmallBubbleImage->icBox.rectLeft;
	
	extern void BlitTest( const DTSOffView *, const DTSImage *,
						  const DTSRect *, const DTSRect * );
	BlitTest( this, &offSmallBubbleImage->icImage.cliImage,
				&offSmallBubbleImage->icBox, &dst );
	}
#endif  // 0
	
#if 1
	//
	// now purge some cache if the image count is too high
	// avoids memory gluttony under OS X
	//
	if ( true /* gUsingOpenGL */ )
		{
		const uint maxcache = 400;	// wild guess
		if ( gRootCacheObject->sCachedImageCount > maxcache )
			gRootCacheObject->RemoveOneObject();
		}
#endif	// 1
	
	// reset the port
	Reset();
}


/*
**	CLOffView::DrawQueuedPictures()
**
**	draw the pictures in the queue up to but not including
**	the specified plane
*/
void
CLOffView::DrawQueuedPictures( int stopPlane )
{
	// start drawing the pictures
	PictureQueue * pq = gPicQueStart;
	int count = gPicQueCount;
	for ( ;  count > 0;  --count, ++pq )
		{
		if ( pq->pqPlane >= stopPlane )
			break;
		
		Draw1Picture( pq );
		}
	
	// save the pointer to the remainder of the queue
	// and the count remaining in the queue
	gPicQueStart = pq;
	gPicQueCount = count;
}


/*
**	CLOffView::RebuildNightList()
**
**	broken out of Draw1Picture() for sanity's sake
*/
void
CLOffView::RebuildNightList( const DTSRect& dst )
{
	int nightTargetLevel = gNightInfo.GetLevel();
	int limit = gNightInfo.GetEffectiveNightLimit();
	if ( nightTargetLevel > limit )
		nightTargetLevel = limit;
	
//	GLfloat clearColorf = (100 - nightTargetLevel) / 100.0f;
	
# ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
	if ( gUsingOpenGL )
		{
# endif  // OGL_QUICKDRAW_LIGHT_EFFECTS
	enableBlending();
	disableAlphaTest();		// experiment with this one...
	disableTexturing();
	
	if ( not nightList )	// this should only get called once
		{
		nightList = glGenLists( 1 );
		nightListLevel = 0;			// redundant?
		nightListRedshift = -1.0f;	// redundant?
		}
	
	if ( nightList
			&&
		(
			not gOGLNightTexture
				||
			not ( gNumLightCasters || gNumDarkCasters)
//				||
//			( nightTargetLevel == 0 ) && not gNumDarkCasters )
		)
	) {
		if ( nightTargetLevel )	// only if there is any night to draw...
			{
			GLfloat nightTargetRedshift = gNightInfo.GetMorningEveningRedshift();
				// if the night level in the displaylist isn't
				// the night level we want to show
				// or if the redshift has changed
			if ( ( nightListLevel != (uint) nightTargetLevel)
					||
				( nightListRedshift != nightTargetRedshift ))
			{
				nightListRedshift = nightTargetRedshift;
				nightListLevel = nightTargetLevel;
				GLfloat nightListLevelf = nightListLevel / 100.0f;
				
				// textured night will use color blend (alpha ignored)
				
				GLfloat rimColor = 1.0f - nightListLevelf;
					// note that the maximum center color is reduced to 0.5
					// (down from 1.0) because the remainder is expected to come
					// from exile lighting. in this special case though, there
					// are neither light nor dark casters
				GLfloat centerColor;
				if ( nightListLevelf < 0.5f )
					centerColor = rimColor;
				else
					centerColor = 0.5f;
				
				glNewList( nightList, GL_COMPILE_AND_EXECUTE );
				
					glPushMatrix();
							// by moving this inside the display list,
							// we are assuming that dst will always be the same
						glTranslatef( (dst.rectRight + dst.rectLeft) * 0.5f,
									  (dst.rectBottom + dst.rectTop) * 0.5f, 0.0f );
						glScalef( 325.0f, 325.0f, 1.0f );
						// this amounts to  dest * source; alpha doesn't matter
						glBlendFunc( GL_DST_COLOR, GL_ZERO );
						if ( gOGLUseDither )
							glEnable( GL_DITHER );
						
							// center circle
						glBegin( GL_TRIANGLE_FAN );
							glColor4f( centerColor, centerColor, centerColor, 0.0f );
							glVertex2f( 0.0f, 0.0f );
						
							glColor4f( rimColor * nightListRedshift,
								rimColor, rimColor, nightListLevelf );
							
							for ( int i = kUnitCircleSlices; i > 0; --i )
								glVertex2fv( kUnitCircle[i - 1] );
							
							glVertex2fv( kUnitCircle[kUnitCircleSlices - 1] );
						glEnd();
						
							// bottom right
						glBegin( GL_TRIANGLE_FAN );
							glVertex2f( 1.0f, 1.0f );
							
							for ( int i = 0; i <= kUnitCircleSlices / 4; ++i )
								glVertex2fv( kUnitCircle[i] );
						glEnd();
						
							// bottom left
						glBegin( GL_TRIANGLE_FAN );
							glVertex2f( -1.0f, 1.0f );
						
							for ( int i = kUnitCircleSlices / 4;
									i <= kUnitCircleSlices / 2; ++i )
								glVertex2fv( kUnitCircle[i] );
						glEnd();
						
							// top left
						glBegin( GL_TRIANGLE_FAN );
							glVertex2f( -1.0f, -1.0f );
							
							for ( int i = kUnitCircleSlices / 2;
									i <= 3 * kUnitCircleSlices / 4; ++i )
								glVertex2fv( kUnitCircle[i] );
						glEnd();
						
							// top right
						glBegin( GL_TRIANGLE_FAN );
							glVertex2f( 1.0f, -1.0f );
							
							for ( int i = 3 * kUnitCircleSlices / 4;
									 i < kUnitCircleSlices; ++i )		// NOT <=
								glVertex2fv( kUnitCircle[i] );
							
							glVertex2fv( kUnitCircle[0] );
						glEnd();
						
						if ( gOGLUseDither )
							glDisable( GL_DITHER );
						
						// reset to what i had been using
						glBlendFunc( GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA );
					glPopMatrix();
				glEndList();
				}
			else
				glCallList( nightList );
			}
		}
# ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
		}
	else
		{
// this probably doesn't work, but can't be properly tested until
// we start to implement exile-with-no-lights
		ushort clearColorus = clearColorf * 0xFFFF;
		GLfloat redshiftClearColorf = clearColorf
									* gNightInfo.GetMorningEveningRedshift();
		if ( redshiftClearColorf > 1.0f )
			redshiftClearColorf = 1.0f;
		ushort redshiftus = redshiftClearColorf * 0xFFFF;
		
		if ( clearColorf >= 0.5f )
			{
			DTSColor background( redshiftus, clearColorus, clearColorus );
			::RGBBackColor( DTS2Mac(&background) );
			::EraseRect( DTS2Mac(&gLayout.layoFieldBox) );
			::RGBBackColor( DTS2Mac(&DTSColor::white) );
			}
		else
			{
			GLfloat starlightColorf = 0.5f - clearColorf;
			ushort starlightColorus = 0xFFFF * starlightColorf;
			RGBColor starlightColorusv =
				{ starlightColorus, starlightColorus, starlightColorus };
			Rect box = { gFieldCenter.ptV - 325, gFieldCenter.ptH - 325,
						 gFieldCenter.ptV + 325, gFieldCenter.ptH + 325 };
			RGBColor moonlightColorusv = { redshiftus, clearColorus, clearColorus };
			qdClearLightmap( moonlightColorusv, starlightColorusv );
			}
		}
# endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
}


/*
**	CLOffView::Draw1Picture()
*/
void
CLOffView::Draw1Picture( const PictureQueue * pq )
{
	// just being paranoid
	ImageCache * cache = pq->pqCache;
	if ( not cache )
		return;
	
#if defined(USE_OPENGL) && defined(DEBUG_VERSION) && defined(OGL_SHOW_IMAGE_ID)
	// for finding problem images when using light casters
	ShowMessage( "pict ID: %d", (int) cache->icImage.cliPictDefID );
#endif	// USE_OPENGL && DEBUG_VERSION && OGL_SHOW_IMAGE_ID
	
	// if shadows aren't supposed to show up in this location, just return
	if ( (cache->icImage.cliPictDef.pdFlags & kPictDefIsShadow)
	&&   0 == GetShadowLevel() )
		{
		return;
		}
	
	// draw shadows different at night
	bool imageIsShadow = false;
	if ( gNightInfo.GetLevel() > 33
	&&	 gNightInfo.GetEffectiveNightLimit() > 33 )
		{
		imageIsShadow = (cache->icImage.cliPictDef.pdFlags & kPictDefIsShadow) != 0;
		}
	
	// the coordinates in the DSPicture record are of the center of the image
	// relative to the center of the screen.
	DTSRect dst;
	int cacheHt = cache->icHeight;
	int cacheWd = cache->icBox.rectRight;
	
	dst.rectTop    = pq->pqVert + gFieldCenter.ptV - cacheHt / 2;
	dst.rectLeft   = pq->pqHorz + gFieldCenter.ptH - cacheWd / 2;
	dst.rectBottom = dst.rectTop  + cacheHt;
	dst.rectRight  = dst.rectLeft + cacheWd;
	
#ifdef PLANE_ZERO_SHADOWS
	if ( 0 == pq->pqPlane )
		{
# ifdef USE_OPENGL
		CreateShadow( this, cache, &cache->icBox, &dst, &gFieldCenter,
			kPictDefShadowNormal /* cache->icImage.cliPictDef.pdFlags */ );
# else
		CreateShadow( this, &cache->icImage.cliImage,
			&cache->icBox, &dst, &gFieldCenter,
			kPictDefShadowNormal /* cache->icImage.cliPictDef.pdFlags */ );
# endif  // USE_OPENGL
		}
#endif  // PLANE_ZERO_SHADOWS
	
	// select the transfer mode
	uint flags = cache->icImage.cliPictDef.pdFlags &
					(kPictDefFlagTransparent | kPictDefBlendMask);
	if ( not gPrefsData.pdFastBlitters
	&&   not gFastBlendMode )
		{
		flags |= kPictDefQualityMode;
		}
	if ( imageIsShadow )
		{
		flags &= ~ kPictDefBlendMask;
		flags |= kPictDef75Blend;
		}
	
	// push random anims along _before_ drawing
#ifdef IRREGULAR_ANIMATIONS
	if ( cache->icImage.cliPictDef.pdFlags & kPictDefFlagRandomAnimation )
		cache->UpdateAnim( true );
#endif	// IRREGULAR_ANIMATIONS
	
#ifdef USE_OPENGL
	// this if() is fast enough only because we are looking for just 4 id's
	// if the number were a little bigger we might get away with a switch
	// but the real long-term solution will probably be an array/table lookup
	// that we could use to determine if we need to draw a particular sprite
	// using a quad, or to enhance/override it somehow with some OpenGL stuff
	
	if (
# ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
		( not gUsingOpenGL || gOpenGLEnableEffects )
# else
		gUsingOpenGL && gOpenGLEnableEffects
# endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
			&&
		(	(cache->icImage.cliPictDefID == kNight25Pct)
				||
			(cache->icImage.cliPictDefID == kNight50Pct)
				||
			(cache->icImage.cliPictDefID == kNight75Pct)
				||
			(cache->icImage.cliPictDefID == kNight100Pct)
		)
	  )
		{
		RebuildNightList( dst );
		}
	else
	if ( gUsingOpenGL
	&&   gOpenGLEnableEffects
	&&   not gOGLDisableShadowEffects
	&&   gOGLStencilBufferAvailable
	&&   cache->icImage.cliPictDef.pdFlags & kPictDefIsShadow )
		{
		drawOGLPixmapAsTexture(
			cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
			kIndexToAlphaMapAlpha100TransparentZero,
			cache->icImage.cliPictDef.pdNumFrames > 1
				? TextureObject::kNAnimatedTiles : TextureObject::kSingleTile,
			cache );
		}
	else
#endif	// USE_OPENGL
		{
		switch ( flags )
			{
			case kPictDefNoBlend:
			case kPictDefNoBlend+kPictDefQualityMode:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapNone,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					Draw( &cache->icImage.cliImage,
						kImageCopyFast, &cache->icBox, &dst );
				break;
			
			case kPictDefFlagTransparent:
			case kPictDefFlagTransparent+kPictDefQualityMode:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha100TransparentZero,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					BlitTransparent( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDef25Blend:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha75,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					FastBlitBlend25( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDef50Blend:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha50,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					FastBlitBlend50( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDef75Blend:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha25,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					FastBlitBlend75( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDefFlagTransparent + kPictDef25Blend:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha75TransparentZero,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					FastBlitBlend25Transparent( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDefFlagTransparent + kPictDef50Blend:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha50TransparentZero,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					FastBlitBlend50Transparent( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDefFlagTransparent + kPictDef75Blend:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha25TransparentZero,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					FastBlitBlend75Transparent( this,
						&cache->icImage.cliImage, &cache->icBox, &dst );
				break;
			
			case kPictDef25Blend + kPictDefQualityMode:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha75,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache);
				else
#endif	// USE_OPENGL
					QualityBlitBlend25( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDef50Blend + kPictDefQualityMode:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha50,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					QualityBlitBlend50( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDef75Blend + kPictDefQualityMode:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha25,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					QualityBlitBlend75( this, &cache->icImage.cliImage,
						&cache->icBox, &dst );
				break;
			
			case kPictDefFlagTransparent + kPictDef25Blend + kPictDefQualityMode:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha75TransparentZero,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					QualityBlitBlend25Transparent( this,
						&cache->icImage.cliImage, &cache->icBox, &dst );
				break;
			
			case kPictDefFlagTransparent + kPictDef50Blend + kPictDefQualityMode:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha50TransparentZero,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					QualityBlitBlend50Transparent( this,
						&cache->icImage.cliImage, &cache->icBox, &dst );
				break;
			
			case kPictDefFlagTransparent + kPictDef75Blend + kPictDefQualityMode:
#ifdef USE_OPENGL
				if ( gUsingOpenGL )
					drawOGLPixmapAsTexture(
						cache->icBox, dst, cache->icImage.cliImage.GetRowBytes(),
						kIndexToAlphaMapAlpha25TransparentZero,
						cache->icImage.cliPictDef.pdNumFrames > 1
							? TextureObject::kNAnimatedTiles
							: TextureObject::kSingleTile,
						cache );
				else
#endif	// USE_OPENGL
					QualityBlitBlend75Transparent( this,
						&cache->icImage.cliImage, &cache->icBox, &dst );
				break;
			}
		}
}


/*
**	CLOffView::SortMobiles()
**
**	sort all of the mobiles from top to bottom
**	so that if the mobiles overlap at all, we always draw their
**	little heads on top of their little feets.
*/
void
CLOffView::SortMobiles()
{
	// don't bother if there is only one mobile on the screen
	int count = gNumMobiles;
	if ( count <= 1 )
		return;
	
	// insert each mobile in the list
	DSMobile * start  = gDSMobile;
	DSMobile * insert = start;
	for ( int done = 1;  done < count;  ++done )
		{
		// save the info to be inserted
		int index  = insert[1].dsmIndex;
		int state  = insert[1].dsmState;
		int horz   = insert[1].dsmHorz;
		int vert   = insert[1].dsmVert;
		int colors = insert[1].dsmColors;
		
		// find where it goes
		DSMobile * test = insert;
		for ( int nnn = done;  nnn > 0;  --nnn )
			{
			// stop when we find a player at or above this one.
			if ( test->dsmVert <= vert )
				break;
			
			// move the hole in the list up one slot
			test[1] = test[0];
			
			// go up the list
			--test;
			}
		
		// save the info for the new player
		test[1].dsmIndex  = index;
		test[1].dsmState  = state;
		test[1].dsmHorz   = horz;
		test[1].dsmVert   = vert;
		test[1].dsmColors = colors;
		
		// insert the next player
		++insert;
		}
}


/*
**	CLOffView::DrawMobileShadows()
**
**	Draw shadows for all of the mobiles in the list.
*/
void
CLOffView::DrawMobileShadows()
{
	DSMobile * mobile = gDSMobile;
	for ( int nnn = gNumMobiles;  nnn > 0;  --nnn )
		{
		Draw1MobileShadow( mobile );
		++mobile;
		}
}


/*
**	CLOffView::Draw1MobileShadow()
**
**	Draw one mobile's shadow
*/
void
CLOffView::Draw1MobileShadow( const DSMobile * mobile )
{
	DescTable * desc = gDescTable + mobile->dsmIndex;
	
	// find an image for this mobile
	ImageCache * ic = CachePicture( desc );
	if ( not ic )
		return;
	
	// the image is 16 poses wide
	int mobileSize = ic->icBox.rectRight / 16;
	int halfSize   = mobileSize / 2;
	
	DTSRect srcBox;
	int state = mobile->dsmState;
	srcBox.rectTop  = ic->icBox.rectTop + mobileSize * (state >> 4);
	srcBox.rectLeft = mobileSize * (state & 0x0F);
	if ( ic->icImage.cliPictDef.pdFlags & kPictDefCustomColors )
		++srcBox.rectTop;
	
	srcBox.Size( mobileSize, mobileSize );
	
	DTSRect dstBox;
	dstBox.rectTop  = mobile->dsmVert - halfSize + gFieldCenter.ptV;
	dstBox.rectLeft = mobile->dsmHorz - halfSize + gFieldCenter.ptH;
	dstBox.Size( mobileSize, mobileSize );
	
	desc->descLastDstBox = dstBox;
	
#ifndef USE_OPENGL
# ifdef NEW_OVAL_HILITE
	if ( kDescPlayer == desc->descType
	&&   gSelectedPlayerName[0]
	&&	 0 == strcmp( gSelectedPlayerName, desc->descName ) )
		{
		// If the player is selected, draw something special
		DrawSelectionHilite( dstBox, mobileSize, state );
		}
# endif
#endif	// ! USE_OPENGL
	
	// skip the shadow drawing if your machine is slow
	// (can't bail any earlier because we need to cache descLastDstBox)
	// also skip the shadows if the area flags say so
	if ( gPrefsData.pdFastBlitters
	||	 gFastBlendMode
	||	 (gFrame->mLightFlags & kLightNoShadows) )
		{
		return;
		}
	
	DTSRect icBounds;
	ic->icImage.cliImage.GetBounds( &icBounds );
	if ( srcBox.rectLeft   >= 0
	&&   srcBox.rectTop    >= 0
	&&   srcBox.rectRight  <= icBounds.rectRight
	&&   srcBox.rectBottom <= icBounds.rectBottom )
		{
		if ( ic->icImage.cliPictDef.pdFlags & kPictDefFlagUprightShadow )
			{
			int whichPose = ChooseShadowPose( state );
			// temporary hack to not display shadows for /pose dead or lie
			if ( whichPose >= 0 )
				{
				DTSRect shadowRect;
				
				// if the shadow pose is different, figure its location in the picture
				if ( whichPose != state )
					{
					shadowRect.rectTop  = ic->icBox.rectTop
										+ mobileSize * (whichPose >> 4);
					shadowRect.rectLeft = mobileSize * (whichPose & 0x0F);
					if ( ic->icImage.cliPictDef.pdFlags & kPictDefCustomColors )
						++shadowRect.rectTop;
					shadowRect.Size( mobileSize, mobileSize );
					}
				else
					{
					// we've already calculated the source location
					shadowRect = srcBox;
					}
				
				// draw shadow using rotated, pose-shifted picture
#ifdef USE_OPENGL
				CreateShadow( this, ic, &shadowRect, &dstBox,
					kPictDefShadowNormal /* ic->icImage.cliPictDef.pdFlags */ );
#else
				CreateShadow( this, &ic->icImage.cliImage, &shadowRect, &dstBox,
					kPictDefShadowNormal /* ic->icImage.cliPictDef.pdFlags */ );
#endif	// USE_OPENGL
				
#if defined(USE_OPENGL) && defined(DEBUG_VERSION) && defined(OGL_SHOW_IMAGE_ID)
	ShowMessage( "upright shadow mobile ID: %d, state: %d",
		(int) desc->descID, whichPose );
#endif	// USE_OPENGL && DEBUG_VERSION && OGL_SHOW_IMAGE_ID
				}
			else
				{
				// dead or prone players draw drop-shadows
#ifdef USE_OPENGL
				CreateShadow( this, ic, &srcBox, &dstBox,
					kPictDefShadowDrop /* ic->icImage.cliPictDef.pdFlags */ );
#else
				CreateShadow( this, &ic->icImage.cliImage, &srcBox, &dstBox,
					kPictDefShadowDrop /* ic->icImage.cliPictDef.pdFlags */ );
#endif	// USE_OPENGL
	
#if defined(USE_OPENGL) && defined(DEBUG_VERSION) && defined(OGL_SHOW_IMAGE_ID)
	ShowMessage( "drop shadow mobile ID: %d", (int) desc->descID );
#endif	// USE_OPENGL && DEBUG_VERSION && OGL_SHOW_IMAGE_ID
				}
			}
		else
			{
			// boats, belly-crawlers & 4-leggers draw drop-shadows
#ifdef USE_OPENGL
			CreateShadow( this, ic, &srcBox, &dstBox,
				kPictDefShadowDrop /* ic->icImage.cliPictDef.pdFlags */ );
#else
			CreateShadow( this, &ic->icImage.cliImage, &srcBox, &dstBox,
				kPictDefShadowDrop /* ic->icImage.cliPictDef.pdFlags */ );
#endif	// USE_OPENGL
			}
		}
}


#ifdef NEW_OVAL_HILITE

/*
**	SelectionView is an OffView into which we composite an image
**	for indicating the selected player, prior to blitting it translucently
**	onto the screen
*/
class SelectionView : public DTSOffView
{
public:
	virtual void DoDraw();
};

/*
**	SelectionView::DoDraw()
**
**	build the image offscreen
*/
void
SelectionView::DoDraw()
{
	DTSRect box;
	GetBounds( &box );
	Erase( &box );
	
	// draw the animated yin-yang thing/thang (yellow/blue)
	const DTSColor color1( 0xFFFF, 0xFFFF, 0 ), color2( 0, 0, 0xFFFF );
	
	DTSArc wedge;
	wedge.Set( box.rectLeft, box.rectTop, box.rectRight, box.rectBottom );
	
	int counter = (gFrameCounter >> 1) % 24;
	int angle = counter * 15;
	
	// then paint the bases
		// 1st half (cover entire circle to eliminate gaps)
	SetForeColor( &color1 );
	wedge.SetAngles( angle, angle + 180 );
	Paint( static_cast<DTSOval *>( static_cast<DTSRect *>( &wedge ) ) );
	
		// 2nd half
	SetForeColor( &color2 );
	wedge.SetAngles( angle + 180, angle + 360 );
	Paint( &wedge );
	
	// paint the half-circles
	// midpoints
	int xmid = (box.rectLeft + box.rectRight) / 2;
	int ymid = (box.rectTop + box.rectBottom) / 2;
	
	// cheap sines/cosines
	const int kScale = 1000;
	static const DTSCoord SinCos[] =
		{
		0,		259,  500,  707,  866,  966,		// 0-90
		1000, 	966,  866,  707,  500,  259,		// 90-180
		0,	   -259, -500, -707, -866, -966,		// 180-270
		-1000, -966, -866, -707, -500, -259,		// 270-360
		0,		259,  500,  707,  866,	966			// 0-90, for cos wrap
		};
	
	int theSin = SinCos[ counter ];
	int theCos = SinCos[ 6 + counter ];
	
	// radii
	int major = (box.rectRight - box.rectLeft) / 4;
	int minor = (box.rectBottom - box.rectTop) / 4;
	
	// coords of small oval centers
	int h1 = xmid - theSin * major / kScale;
	int v1 = ymid + theCos * minor / kScale;
	
	int h2 = xmid + xmid - h1;
	int v2 = ymid + ymid - v1;
	
	DTSOval w2;
	w2.Set( h1 - major, v1 - minor, h1 + major, v1 + minor );
	SetForeColor( &color1 );
	Paint( &w2 );
	
	SetForeColor( &color2 );
	w2.Set( h2 - major, v2 - minor, h2 + major, v2 + minor );
	Paint( &w2 );
	
	SetForeColor( &DTSColor::black );
}


/*
**	CLOffView::DrawSelectionHilite()
**
**	draw some kind of indicator for the selected player
*/
#ifdef USE_OPENGL
void
CLOffView::DrawSelectionHilite()
{
	const DSMobile * mobile = gDSMobile;
	for ( int nnn = gNumMobiles;  nnn > 0;  --nnn, ++mobile )
		{
		DescTable * desc = gDescTable + mobile->dsmIndex;
		
		// find an image for this mobile
		ImageCache * ic = CachePicture( desc );
		if ( not ic )
			continue;
		
		// the image is 16 poses wide
		int mobileSize = ic->icBox.rectRight / 16;
		int halfSize   = mobileSize / 2;
		
		int state = mobile->dsmState;
		
		DTSRect dstBox;
		dstBox.rectTop  = mobile->dsmVert - halfSize + gFieldCenter.ptV;
		dstBox.rectLeft = mobile->dsmHorz - halfSize + gFieldCenter.ptH;
		dstBox.Size( mobileSize, mobileSize );
		
		desc->descLastDstBox = dstBox;
		
		if ( kDescPlayer == desc->descType
		&&   gSelectedPlayerName[0]
		&&	 0 == strcmp( gSelectedPlayerName, desc->descName ) )
			{
			// If the player is selected, draw something special
			DrawSelectionHilite( dstBox, mobileSize, state );
			return;
			}
	}
}


/*
**	DrawGLHilite()
**
**	build 'gOGLSelectionList', the display-list for a player hilite
**	then call it
*/
static void
DrawGLHilite( const DTSRect& box, int& openGLEnableEffectsState )
{
	int counter = (gFrameCounter >> 1) % 24;
	int angle = counter * 15;
	GLfloat centerH = float( box.rectLeft + box.rectRight ) * 0.5f;
	GLfloat centerV = float( box.rectTop + box.rectBottom ) * 0.5f;
	
	glPushMatrix();
		glTranslatef( centerH, centerV, 0.0f );
		GLdouble hRadius = box.rectRight - box.rectLeft;
		if ( hRadius > 0.0 )
			{
			GLdouble aspect = (box.rectBottom - box.rectTop) / hRadius;
				// doing this with scale instead of second rotation,
				// because gluOrtho2D clips at +/-1 (z-axis)
			glScaled( 1.0, aspect, 1.0 );
			}
		hRadius *= 0.5f;
		glRotatef( angle, 0.0f, 0.0f, 1.0f );
		
		glScaled( hRadius, -hRadius, 1.0 );
		
		enableBlending();
		disableAlphaTest();
		disableTexturing();
		disableStencilTest();
		
		if ( openGLEnableEffectsState != int(gOpenGLEnableEffects) )
			{
			openGLEnableEffectsState = int(gOpenGLEnableEffects);
			GLfloat inner[17][2], outer[17][2];
			
			// this is a lot easier to understand
			// if you plot a few points on some graph paper
			for ( int i = 0; i <= 11; ++i )
				{
				int index = i + 5;
				outer[index][0] = cosf( i * 15 * F_PI / 180.0f );
				outer[index][1] = sinf( i * 15 * F_PI / 180.0f );
				
				inner[index][0] = outer[index][0] * 0.5f - 0.5f;
				inner[index][1] = outer[index][1] * 0.5f;
				}
			for ( int i = 0; i <= 4; ++i )
				{
				outer[i][0] = - inner[i + 12][0];
				outer[i][1] = - inner[i + 12][1];
				
				inner[i][0] = - inner[10 - i][0];
				inner[i][1] = - inner[10 - i][1];
				}
			
			glNewList( gOGLSelectionList, GL_COMPILE_AND_EXECUTE );
				if ( not openGLEnableEffectsState )
					glColor4us( 0x0, 0x0, 0xFFFF, 0x7fff );
				
				glBegin( GL_TRIANGLE_STRIP );
					if ( openGLEnableEffectsState )
						glColor4us( 0x0, 0x0, 0xFFFF, 0xFFFF );
					
					glVertex2f( 0.5f, -0.5f );
					
					for ( int i = 0; i <= 16; ++i )
						{
						if ( openGLEnableEffectsState )
							glColor4us( 0x0, 0x0, 0xFFFF,
								0xFFFF * (1.0f - (i + 1) / 18.0f) );
						
						glVertex2fv( inner[i] );
						glVertex2fv( outer[i] );
						}
					
					if ( openGLEnableEffectsState )
						glColor4us( 0x0, 0x0, 0xFFFF, 0x0 );
					
					glVertex2f( -1.0f, 0.0f );
				glEnd();
				
				glRotatef( 180.0f, 0.0f, 0.0f, 1.0f );
				
				if ( not openGLEnableEffectsState )
					glColor4us( 0xFFFF, 0xFFFF, 0x0, 0x7fff );
				
				glBegin( GL_TRIANGLE_STRIP );
					if ( openGLEnableEffectsState )
						glColor4us( 0xFFFF, 0xFFFF, 0x0, 0xFFFF );
					
					glVertex2f( 0.5f, -0.5f );
					
					for ( int i = 0; i <= 16; ++i )
						{
						if ( openGLEnableEffectsState )
							glColor4us( 0xFFFF, 0xFFFF, 0,
								0xFFFF * (1.0f - (i + 1) / 18.0f) );
						
						glVertex2fv( inner[i] );
						glVertex2fv( outer[i] );
						}
					
					if ( openGLEnableEffectsState )
						glColor4us( 0xFFFF, 0xFFFF, 0x0, 0x0 );
					
					glVertex2f( -1.0f, 0.0f );
				glEnd();
			glEndList();
			}
		else
			glCallList( gOGLSelectionList );
		
	glPopMatrix();
}
#endif	// USE_OPENGL


/*
**	CLOffView::DrawSelectionHilite()
*/
void
CLOffView::DrawSelectionHilite( DTSRect box, int mobileSize, int state )
{
	box.rectTop += (mobileSize + mobileSize) / 3;
	box.rectBottom += mobileSize / 4;
	box.Inset( -4, -2 );
	
	// if they're not on their feet, nudge the indicator upward to match
	if ( kPoseLie == state
	||	 kPoseDead == state )
		{
		box.Offset( 0, 1 - mobileSize / 8 );
		}
	else
		box.Offset( 0, -4 );	
	
#ifdef USE_OPENGL
	// static probably isn't the most elegant way to do this
	static int openGLEnableEffectsState = -1;
	if ( gUsingOpenGL && not gOGLSelectionList )
		{
		if ( not gOGLSelectionList )
			{
			gOGLSelectionList = glGenLists( 1 );
			if ( gOGLSelectionList )
				openGLEnableEffectsState = -1;
			}
		}
	if ( gUsingOpenGL && gOGLSelectionList )
		DrawGLHilite( box, openGLEnableEffectsState );
	else
#endif	// USE_OPENGL
		{
		DTSImage select;
		select.DisposeBits();
		DTSError result = select.Init( nullptr, box.rectRight - box.rectLeft,
									   box.rectBottom - box.rectTop, kCLDepth );
		if ( noErr == result )
			result = select.AllocateBits();
		
		if ( noErr == result )
			{
			SelectionView selView;
			selView.Init( kCLDepth );
			selView.Hide();
			selView.SetImage( &select );
			selView.Show();
			
//			Draw( &select, kImageTransparent, &select.imageBounds, &box );
			DTSRect selectBounds;
			select.GetBounds( &selectBounds );
			SelectiveBlit( this, &select, &selectBounds, &box, kBlitter50 );
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				drawOGLPixmap( selectBounds, box, select.GetRowBytes(),
						kIndexToAlphaMapAlpha50TransparentZero, select.GetBits() );
			else
#endif	// USE_OPENGL
				SelectiveBlit( this, &select, &selectBounds, &box, kBlitter50 );
			}
		}
}
#endif	// NEW_OVAL_HILITE


/*
**	CLOffView::DrawNames()
**
**	Draw the names of all of the mobiles in the list.
*/
void
CLOffView::DrawNames()
{
	DSMobile * mobile = gDSMobile;
	
	for ( int nnn = gNumMobiles;  nnn > 0;  --nnn )
		{
		Draw1Name( mobile );
		++mobile;
		}
}


/*
**	CLOffView::Draw1Name()
**
**	Draw one of the names.
*/
void
CLOffView::Draw1Name( const DSMobile * mobile )
{
	DescTable * desc = gDescTable + mobile->dsmIndex;
	
	uint style = (mobile->dsmColors & 0x003);
	
	const DTSColor * backClr =
			&gBackColorTable[ (mobile->dsmColors & 0x0F0) >> kColorCodeBackShift ];
	const DTSColor * textClr =
			&gTextColorTable[ (mobile->dsmColors & 0x00C) >> kColorCodeTextShift ];
	
	int descType = desc->descType;
	
	int friends = kFriendNone;
	if ( kDescPlayer == descType )		// only players can be friends
		friends = GetPlayerIsFriend( desc );
	
	// if the person is being fully ignored, don't draw their name.
	if ( kFriendIgnore == friends )
		return;
	
	// hide names by user preference
	const char * name = desc->descName;
#ifndef AUTO_HIDENAME
	if ( not gPrefsData.pdShowNames )
		{
		// the user doesn't want to see any names
		name = nullptr;
		}
#endif  // ! AUTO_HIDENAME
	
	// override server name state for monsters
	// _iff_ someone turned on the hide-monster-name pref via the GM client
	// regular clients have no access to that setting, so they'll never have it
	// on. So players will never see monster names unless a script specifically
	// wants them to. GMs can change the pref if desired (via the debug client);
	// the value will persist even unto future clients, both regular and debug.
	// Therefore GMs will either see all names, or no names.
	// Come to think of it, the really flexible way to handle all this is at the
	// server, via an "/options" flag that's consulted when assembling the
	// drawstates.
	// Oh well.
	if ( kDescMonster == descType
	&&   gPrefsData.pdGodHideMonsterNames )
		{
		// the GM doesn't want to see monster names
		name = nullptr;
		}
	
#ifdef AUTO_HIDENAME
	if ( descType != kDescMonster
	&&	 descType != kDescThoughtBubble
	&&	 name && *name )
		{
		if ( not desc->descSeenFrame )		// not seen him, or long ago
			{
			desc->descNameVisible = kName_Visible;
			desc->descSeenFrame = gFrameCounter + kNameDelay_First;
				// show him full for a "long" time
			}
		else
			{
			if ( desc->descSeenFrame <= gFrameCounter )
				{
				// trigger
				if ( desc->descNameVisible > kName_Hidden )
					{
					// still visible ?
					--desc->descNameVisible;	// "less" visible
					// for some time
					desc->descSeenFrame = gFrameCounter + kNameDelay_Hide;
					}
				// If it's been a "long" time: force the name to be drawn
				if ( gFrameCounter - desc->descSeenFrame > kNameDelay_MemoryRefresh )
					desc->descSeenFrame = 0;
				}
			}
		
		// do auto-hide behavior unless the pref says don't
		if ( kName_Hidden == desc->descNameVisible && gPrefsData.pdShowNames )
			name = nullptr;
		}
#endif	// AUTO_HIDENAME
	
	// no name to draw
	// perhaps we should draw an injury bar instead
	if ( not name
	||   0 == *name )
		{
		// NPCs with no names don't get blue bars
		// NPCs with names do get blue bars when show-names is off
		if ( kDescNPC != descType
		||   0        != desc->descName[0] )
			{
			Draw1InjuryBar( mobile, friends );
			}
		return;
		}
	
// note:  i cannot conditionalize the system (nonogl) font commmands
// because i need the font to be correct for the call to
// GetTextWidth() (which calls ::TextWidth())
	
	// set the font style
	int dtsstyle = kStyleNormal;
	if ( style & kColorCodeStyleBold )
		dtsstyle |= kStyleBold;
	if ( style & kColorCodeStyleItalic )
		dtsstyle |= kStyleItalic;
	if ( GetPlayerIsSharee( desc ) )
		dtsstyle |= kStyleUnderline;
	
	SetFontStyle( dtsstyle );
	
	// draw the box frame
	SetFont( "Geneva" );
	SetFontSize( 9 );
	int width = GetTextWidth( name ) + 1;
	switch ( style )
		{
		case kColorCodeStyleItalic:
			width += 1;
			break;
		
		case kColorCodeStyleItalic + kColorCodeStyleBold:
			width += 3;
			break;
		}
	DTSRect box;
	box.rectTop    = mobile->dsmVert + gFieldCenter.ptV + desc->descSize + 2;
	box.rectLeft   = mobile->dsmHorz + gFieldCenter.ptH - (width / 2) - 2;
	box.Size( width + 2 + 2, 2 + 9 + 2 + 2 );
	
	DTSColor frameColor;
	frameColor.SetBlack();
	
	if ( kFriendNone < friends
	&&	 kFriendFlagCount > friends )
		{
		frameColor = gFriendColors[ friends ];
		}
	
	// draw the box around the nametag
#ifdef USE_OPENGL
	
		// Do GMs see a special alpha for monster-name backgrounds?
# ifdef DEBUG_VERSION
#  define FAINT_MONSTERS		1
# else
#  define FAINT_MONSTERS		0
# endif

# if FAINT_MONSTERS
		// custom alpha level for certain nametag backgrounds
	GLushort bgAlpha;
	switch ( descType )
		{
		case kDescMonster:	bgAlpha = 0x4000;	break;
//		case kDescNPC:		bgAlpha = 0x8000;	break;
		default:			bgAlpha = 0xFFFF;	break;
		}
# endif  // FAINT_MONSTERS
	
	if ( gUsingOpenGL )
		{
			// move/enable blending up here?
	
	// v513: some mid-2007 iMacs fail to draw name-box backgrounds & frames if
	// blending is disabled, so we now turn it on throughout (up til drawing the
	// actual text)
# define NAME_BOX_FIX_513	1

# if NAME_BOX_FIX_513 || FAINT_MONSTERS
		enableBlending();
# else
		disableBlending();	// can't enable with no alpha in the color...
# endif  // NAME_BOX_FIX_513
		
		disableAlphaTest();
		disableTexturing();
		
# if FAINT_MONSTERS
		glColor4us( frameColor.rgbRed, frameColor.rgbGreen,
			frameColor.rgbBlue, bgAlpha );
# else
		glColor3us( frameColor.rgbRed, frameColor.rgbGreen,
			frameColor.rgbBlue );
# endif
		glBegin( GL_LINE_LOOP );
			glVertex2f( box.rectLeft + 0.5f, box.rectTop + 0.5f );
			glVertex2f( box.rectRight - 0.5f, box.rectTop + 0.5f );
			glVertex2f( box.rectRight - 0.5f, box.rectBottom - 0.5f );
			glVertex2f( box.rectLeft + 0.5f, box.rectBottom - 0.5f );
		glEnd();
		}
	else
#endif	// USE_OPENGL
		{
		SetForeColor( &frameColor );
		Frame( &box );
		}
	box.Inset( 1, 1 );
	
#ifdef USE_OPENGL
	// choose the appropriate OGL font
	GLuint oglFontListBase = 0;
	if ( gUsingOpenGL )
		{
		switch ( dtsstyle )
			{
			case kStyleBold:
				oglFontListBase = geneva9BoldListBase;
				break;
			
			case kStyleItalic:
				oglFontListBase = geneva9ItalicListBase;
				break;
			
			case kStyleBold | kStyleItalic:
				oglFontListBase = geneva9BoldItalicListBase;
				break;
			
			case kStyleUnderline:
				oglFontListBase = geneva9UnderlineListBase;
				break;
			
			case kStyleBold | kStyleUnderline:
				oglFontListBase = geneva9BoldUnderlineListBase;
				break;
			
			case kStyleItalic | kStyleUnderline:
				oglFontListBase = geneva9ItalicUnderlineListBase;
				break;
			
			case kStyleBold | kStyleItalic | kStyleUnderline:
				oglFontListBase = geneva9BoldItalicUnderlineListBase;
				break;
			
			default:	// kStyleNormal
				oglFontListBase = geneva9NormalListBase;
				break;
			}
		}
#endif	// USE_OPENGL
	
#ifdef AUTO_HIDENAME
	// for the intermediate stages, make the names translucent.
	// but if turned off in the prefs, never blend
	if ( gPrefsData.pdShowNames  &&  kName_Visible != desc->descNameVisible )
		{
# ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
			// i could go through the motions and follow the codepath that is
			// DrawBlendedName() which protects me against future changes, but
			// this is soooo much simpler...
			
#  if ! NAME_BOX_FIX_513 && ! FAINT_MONSTERS
			enableBlending();	// see disableBlending() above...
#  endif  // NAME_BOX_FIX_513
			
			GLushort alpha = 0xFFFF;
			switch ( desc->descNameVisible )
				{
				case kName_Visible25:	alpha = 0x3FFF;		break;
				case kName_Visible50:	alpha = 0x7FFF;		break;
				case kName_Visible75:	alpha = 0xBFFF;		break;
				}
			// draw the background -- see DrawBlendedName()
			glColor4us( backClr->rgbRed, backClr->rgbGreen, backClr->rgbBlue, alpha );
			glRecti( box.rectLeft, box.rectTop, box.rectRight, box.rectBottom );
			
			// draw the text -- see NameBoxView::DoDraw()
			glColor4us( textClr->rgbRed, textClr->rgbGreen, textClr->rgbBlue, alpha );
			drawOGLText( box.rectLeft + 2, box.rectTop + 10, oglFontListBase, name );
			}
		else
# endif	// USE_OPENGL
			{
			DrawBlendedName( &box, desc->descNameVisible,
				backClr, textClr, dtsstyle, name );
			}
		}
	else
#endif  // AUTO_HIDENAME
		{
		// draw the background
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
# if FAINT_MONSTERS
			glColor4us( backClr->rgbRed, backClr->rgbGreen, backClr->rgbBlue, bgAlpha );
# else
			glColor3us( backClr->rgbRed, backClr->rgbGreen, backClr->rgbBlue );
# endif
			glRecti( box.rectLeft, box.rectTop, box.rectRight, box.rectBottom );
			}
		else
#endif	// USE_OPENGL
			{
			SetForeColor( backClr );
			Paint( &box );
			}
		
		// draw the text
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
# if NAME_BOX_FIX_513 || FAINT_MONSTERS
			disableBlending();
# endif  // NAME_BOX_FIX_513
			
			glColor3us( textClr->rgbRed, textClr->rgbGreen, textClr->rgbBlue );
			drawOGLText( box.rectLeft + 2, box.rectTop + 10, oglFontListBase, name );
			}
		else
#endif	// USE_OPENGL
			{
			SetForeColor( textClr );
			Draw( name, box.rectLeft + 2, box.rectTop + 10, teJustLeft );
			}
		}
	
	// reset the port
#ifdef USE_OPENGL
	if ( gUsingOpenGL )
		glColor3us( 0, 0, 0 );
	else
#endif	// USE_OPENGL
		SetForeColor( &DTSColor::black );
	
	SetFontStyle( kStyleNormal );
}


/*
**	class NameBoxView : an offscreen for compositing mobiles' nametags
*/
class NameBoxView : public DTSOffView
{
	const DTSColor * bkClr;
	const DTSColor * txClr;
	const char * name;
	int style;
	
public:
	void	Setup( const DTSColor * inBk, const DTSColor * inTx,
				   int inStyle, const char * inName )
		{
		bkClr = inBk;
		txClr = inTx;
		name = inName;
		style = inStyle;
		};
	
	virtual void	DoDraw();
};


#ifdef AUTO_HIDENAME
/*
**	NameBoxView::DoDraw()
*/
void
NameBoxView::DoDraw()
{
	// draw the background
	SetForeColor( bkClr );
	DTSRect viewBounds;
	GetBounds( &viewBounds );
	Paint( &viewBounds );
	
	// draw the text
	SetFont( "Geneva" );
	SetFontSize( 9 );
	SetFontStyle( style );
	SetForeColor( txClr );
	Draw( name, viewBounds.rectLeft + 2, viewBounds.rectTop + 10, teJustLeft );
};


/*
**	CLOffView::DrawBlendedName()
**
**	draw the faded name box
*/
void
CLOffView::DrawBlendedName( const DTSRect * box, int vis,
	const DTSColor * bkClr, const DTSColor * txClr, int style, const char * name )
{
	DTSImage namebox;
	namebox.DisposeBits();
	DTSError result = namebox.Init( nullptr, box->rectRight - box->rectLeft,
									box->rectBottom - box->rectTop, kCLDepth );
	if ( noErr == result )
		result = namebox.AllocateBits();
	
	if ( noErr == result )
		{
		NameBoxView nameView;
		nameView.Init( kCLDepth );
		nameView.Setup( bkClr, txClr, style, name );
		nameView.Hide();
		nameView.SetImage( &namebox );
		nameView.Show();
		
		int whichBlit;
		switch ( vis )
			{
			case kName_Visible25:	whichBlit = kBlitter75;	break;
			case kName_Visible50:	whichBlit = kBlitter50;	break;
			case kName_Visible75:	whichBlit = kBlitter25;	break;
			
			// no other cases are possible, but...
			default:				whichBlit = kBlitterTransparent;
			}
		DTSRect nameBounds;
		namebox.GetBounds( &nameBounds );
		SelectiveBlit( this, &namebox, &nameBounds, box, whichBlit );
		}
	SetErrorCode( result );
}
#endif	// AUTO_HIDENAME


/*
**	CLOffView::Draw1InjuryBar()
**
**	Draw injury bar for an unnamed mobile
*/
void
CLOffView::Draw1InjuryBar( const DSMobile * mobile, int friends )
{
	// give the players some sort of injury feedback
	int back = mobile->dsmColors >> kColorCodeBackShift;
/*
	switch ( back )
		{
		case kColorCodeBackWhite:		// perfect health
		case kColorCodeBackGreen:		// good but a little hurt
		case kColorCodeBackYellow:		// hurting
		case kColorCodeBackRed:			// near death
		case kColorCodeBackBlue:		// npc
		case kColorCodeBackGrey:		// ghosting god
		case kColorCodeBackCyan:		// god that looks like a monster
		case kColorCodeBackOrange:		// disconnected player or afk player
			break;
		
		// draw nothing
		default:
			return;
		}
*/
	// paint a box showing injury colors
	DescTable * desc = gDescTable + mobile->dsmIndex;
	DTSRect box;
	box.rectTop  = mobile->dsmVert + gFieldCenter.ptV + desc->descSize + 2;
	box.rectLeft = mobile->dsmHorz + gFieldCenter.ptH - 6;
	box.Size( 12, 2 );
	if ( kColorCodeBackWhite == back
	||   kColorCodeBackBlue  == back
	||  (kColorCodeBackBlack == back && kDescMonster == desc->descType) )
		{
		// do nothing if it's a healthy mobile, an NPC, or a dead monster
		}
	else
		{
		const DTSColor * backClr = &gBackColorTable[ back ];
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
# if ! NAME_BOX_FIX_513
			disableBlending();
# else
			enableBlending();
# endif
			
			disableAlphaTest();
			disableTexturing();
			
# if ! NAME_BOX_FIX_513
			glColor3us( backClr->rgbRed, backClr->rgbGreen, backClr->rgbBlue );
# else
			glColor4us( backClr->rgbRed, backClr->rgbGreen, backClr->rgbBlue, 0xFFFF );
# endif
			glRecti( box.rectLeft, box.rectTop, box.rectRight, box.rectBottom );
			}
		else
#endif	// USE_OPENGL
			{
			SetForeColor( backClr );
			Paint( &box );
			}
		}
	
	// frame the box if it's a friend
	if ( kDescPlayer == desc->descType && friends )
		{
		box.Inset( -1, -1 );
		DTSColor frameColor;
		frameColor.SetBlack();
		
		if ( kFriendNone != friends )
			frameColor = gFriendColors[ friends ];
		
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
# if ! NAME_BOX_FIX_513
			disableBlending();
# else
			enableBlending();
# endif
			
			disableAlphaTest();
			disableTexturing();
			
# if ! NAME_BOX_FIX_513
			glColor3us( frameColor.rgbRed, frameColor.rgbGreen,
				frameColor.rgbBlue );
# else
			glColor4us( frameColor.rgbRed, frameColor.rgbGreen,
				frameColor.rgbBlue, 0xFFFF );
# endif
			}
		else
#endif	// USE_OPENGL
			{
			SetForeColor( &frameColor );
			}
		
		if ( back != kColorCodeBackWhite )
			{
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				{
				glBegin( GL_LINE_LOOP );
					glVertex2f( box.rectLeft + 0.5f, box.rectTop + 0.5f );
					glVertex2f( box.rectRight - 0.5f, box.rectTop + 0.5f );
					glVertex2f( box.rectRight - 0.5f, box.rectBottom - 0.5f );
					glVertex2f( box.rectLeft + 0.5f, box.rectBottom - 0.5f );
				glEnd();
				}
			else
#endif	// USE_OPENGL
				{
				Frame( &box );
				}
			}
		else
			{
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				glRecti( box.rectLeft, box.rectTop, box.rectRight, box.rectBottom );
			else
#endif	// USE_OPENGL
				{
				Paint( &box );
				}
			}
		}
	
	// reset the port
#ifdef USE_OPENGL
	if ( not gUsingOpenGL )
#endif	// USE_OPENGL
		SetForeColor( &gTextColorTable[ kColorCodeTextBlack ] );
	
#if defined( USE_OPENGL ) && NAME_BOX_FIX_513
	if ( gUsingOpenGL )
		disableBlending();
#endif  // OGL && NAME_BOX_FIX_513
}


/*
**	CLOffView::DrawMobiles()
**
**	Draw all of the mobiles in the list.
*/
void
CLOffView::DrawMobiles()
{
	DSMobile * mobile = gDSMobile;
	int numMobiles = gNumMobiles;
	
	// paint all fallen mobiles first, underneath all the rest
	for ( ; numMobiles > 0; --numMobiles, ++mobile )
		if ( kPoseDead == mobile->dsmState )
			Draw1Mobile( mobile );
	
	// rewind to top of mobile list
	mobile = gDSMobile;
	numMobiles = gNumMobiles;
	
	PictureQueue * pq = gPicQueStart;
	int numPicts = gPicQueCount;
	
	bool bNeedMobileVert = true;
	bool bNeedPictVert   = true;
	
	int mobileVert = 0;
	int pictVert = 0;
	
	// simultaneously walk both the mobile list and the image list (plane 0 only)
	// "merging" (actually, drawing) them in order of increasing V-coordinates.
	for (;;)
		{
		if ( bNeedMobileVert )
			{
			if ( numMobiles <= 0 )
				break;
			
			mobileVert = mobile->dsmVert;
			bNeedMobileVert = false;
			}
		
		if ( bNeedPictVert )
			{
			// only consider images on plane 0
			if ( numPicts > 0
			&&   0 == pq->pqPlane )
				{
				pictVert = pq->pqVert;
				}
			else
				pictVert = INT_MAX;
			
			bNeedPictVert = false;
			}
		
		if ( mobileVert < pictVert )
			{
			// fallen mobiles have already been drawn
			if ( mobile->dsmState != kPoseDead )
				Draw1Mobile( mobile );
			++mobile;
			--numMobiles;
			bNeedMobileVert = true;
			}
		else
			{
			Draw1Picture( pq );
			++pq;
			--numPicts;
			bNeedPictVert = true;
			}
		}
	
	gPicQueStart = pq;
	gPicQueCount = numPicts;
}


/*
**	CLOffView::Draw1Mobile()
**
**	Draw a single mobile.
*/
void
CLOffView::Draw1Mobile( const DSMobile * mobile )
{
	DescTable * desc = gDescTable + mobile->dsmIndex;
	int friends = kFriendNone;
	bool dead = false;

#ifndef NEW_OVAL_HILITE
	bool selected = false;
#endif
	
	// If this is a player, update any flag info.
	// Also determine whether the player is a friend.
	if ( desc->descNumColors != 0 )
		{
		uint back  = (mobile->dsmColors & 0x0F0) >> kColorCodeBackShift;
		uint text  = (mobile->dsmColors & 0x00C) >> kColorCodeTextShift;
		uint style = (mobile->dsmColors & 0x003);
		
		// Is their name white, or background black? (clues from the server)
		if ( kColorCodeTextWhite == text || kColorCodeBackBlack == back )
			dead = true;
		
		// only do these checks for player descriptors
		if ( kDescPlayer == desc->descType )
			{
			// decipher server hints re deadness, clan, sharing, etc.
			// and update our local player-info DB accordingly.
			SetPlayerIsDead( desc, dead );
			
			SetPlayerIsClan( desc, (style & kColorCodeStyleItalic) != 0 );
			SetPlayerIsSharing( desc, (style & kColorCodeStyleBold) != 0 );
			
// if you add this player descriptor server-side this would be easier.  BMH
//			SetPlayerIsSharee( desc, (style & kColorCodeStyleUnderline) != 0 );
// -- yeah, but there's no available bits left in the style field
			
			friends = GetPlayerIsFriend( desc );
			
#ifndef NEW_OVAL_HILITE
			if ( back != kColorCodeBackBlue
			&&	 gSelectedPlayerName[0]
			&&	 ( 0 == strcmp( gSelectedPlayerName, desc->descName ) ) )
				{
				selected = true;
				}
#endif  // ! NEW_OVAL_HILITE
			}
		}
	
	DescTable fakeColors;
	//
	// We fake a descriptor if the person is being ignored
	// we set its colors to white, and force the cache to look for the proper
	// colored picture
	// The ignored person appears ghost-like.
	// Seems natural to see them because they still can push/pull etc
	//
	if ( kFriendIgnore == friends )
		{
		fakeColors = *desc;
		fakeColors.descCacheID = fakeColors.descID;	// lookup cache properly
		desc = &fakeColors;
		bzero( desc->descColors, sizeof desc->descColors );
		}
	
	// find an image for this mobile
	ImageCache * ic = CachePicture( desc );
	if ( not ic )
		return;
	
#if defined(USE_OPENGL) && defined(DEBUG_VERSION) && defined(OGL_SHOW_IMAGE_ID)
		// for finding problem images when using light casters
	ShowMessage( "mobile ID: %d, state: %d",
		(int) desc->descID, (int) mobile->dsmState );
#endif	// USE_OPENGL && DEBUG_VERSION && OGL_SHOW_IMAGE_ID
	
	// the image is 16 poses wide
	int mobileSize = ic->icBox.rectRight / 16;
//	int halfSize   = mobileSize / 2;
	
	DTSRect srcBox;
	int state = mobile->dsmState;
	srcBox.rectTop  = ic->icBox.rectTop + mobileSize * ( state >> 4 );
	srcBox.rectLeft = mobileSize * ( state & 0xF );
	if ( ic->icImage.cliPictDef.pdFlags & kPictDefCustomColors )
		++srcBox.rectTop;
	srcBox.Size( mobileSize, mobileSize );
	
	DTSRect dstBox;
	if ( desc != &fakeColors )
		dstBox = desc->descLastDstBox;
	else
		{
		// for fakes, use the real desc box
		const DescTable * realDesc = gDescTable + mobile->dsmIndex;
		dstBox = realDesc->descLastDstBox;
		}
	
	DTSRect icBounds;
	ic->icImage.cliImage.GetBounds( &icBounds );
	if ( srcBox.rectLeft   >= 0
	&&   srcBox.rectTop    >= 0
	&&   srcBox.rectRight  <= icBounds.rectRight
	&&   srcBox.rectBottom <= icBounds.rectBottom )
		{
		int blitter = (ic->icImage.cliPictDef.pdFlags & kPictDefBlendMask);
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
			IndexToAlphaMapID mapID;
			switch ( blitter )
				{
				case kPictDef75Blend:
					mapID = kIndexToAlphaMapAlpha25TransparentZero; break;
				case kPictDef50Blend:
					mapID = kIndexToAlphaMapAlpha50TransparentZero; break;
				case kPictDef25Blend:
					mapID = kIndexToAlphaMapAlpha75TransparentZero; break;
				
				default:
				case kPictDefNoBlend:
					mapID = kIndexToAlphaMapAlpha100TransparentZero; break;
				}
			drawOGLPixmapAsTexture(	srcBox, dstBox,
				ic->icImage.cliImage.GetRowBytes(),
				mapID, TextureObject::kMobileArray, ic );
			}
		else
#endif	// USE_OPENGL
			{
			SelectiveBlit( this, &ic->icImage.cliImage, &srcBox, &dstBox, blitter );
//			BlitTransparent( this, &ic->icImage.cliImage, &srcBox, &dstBox );
			}

#ifndef NEW_OVAL_HILITE
		// If the player is selected draw a blue box.
		if ( selected )
			{
			DTSRect box = dstBox;
			box.Inset( -1, -1 );
			
// i haven't written an opengl version for this; is one still needed?
			SetForeColor( &DTSColor::blue );
			Frame( &box );
			SetForeColor( &DTSColor::black );
			}
#endif

// brain damage
#if 0
		if ( mobile->dsmIndex != 0 )
			ShowMessage( "Drawing descriptor %d '%s'.",
				mobile->dsmIndex, desc->descName );
#endif
		}
	else
		{
// brain damage
#if 0
		ShowMessage( " * Bad frame %d in picture %d.",
			state, (int) ic->icImage.cliPictDefID );
#endif
		
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
			disableBlending();
			disableAlphaTest();
			disableTexturing();
			glColor3us( 0xFFFF, 0, 0 );	// pure red
			}
		else
#endif	// USE_OPENGL
			SetForeColor( &DTSColor::red );
		
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
# if 1
			GLdouble hRadius = dstBox.rectRight - dstBox.rectLeft;
			GLdouble aspect = 0.0;
			if ( hRadius > 0.0 )
				aspect = (dstBox.rectBottom - dstBox.rectTop) / hRadius;
			
			hRadius *= 0.5f;
			GLUquadricObj * qobj = gluNewQuadric();
			glPushMatrix();
				glTranslatef( (dstBox.rectRight + dstBox.rectLeft) * 0.5f,
							  (dstBox.rectBottom + dstBox.rectTop) * 0.5f, 0.0 );
				glScaled( 1.0, aspect, 1.0 );
				gluDisk( qobj, 0.0, hRadius, 24, 1 );
			glPopMatrix();
			gluDeleteQuadric( qobj );
			// oops, this would have gotten me a circle, not an oval.
			// since this is just a failure mode, we could go with something faster...
# else
			glRecti( dstBox.rectLeft, dstBox.rectTop,
				dstBox.rectRight, dstBox.rectBottom );
# endif  // 1
			}
		else
#endif	// USE_OPENGL
			{
			DTSOval oval;
			oval.Set( dstBox.rectLeft, dstBox.rectTop,
				dstBox.rectRight, dstBox.rectBottom );
			Paint( &oval );
			}
		
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			glColor3us( 0, 0, 0 );
		else
#endif	// USE_OPENGL
			SetForeColor( &DTSColor::black );
		}
}


/*
**	CLOffView::DrawBubbles()
**
**	Draw all of the text boxes in the list.
*/
void
CLOffView::DrawBubbles()
{
	// keep track of yellers
	char drawTable[ kDescTableSize ];
	bzero( drawTable, sizeof drawTable );
	
	// prescan all drawable bubbles
	DSMobile * dsm = &gDSMobile[0];
	DescTable * table = gDescTable;
	for ( int count = gNumMobiles;  count > 0;  --count )
		{
		int index = dsm->dsmIndex;
		DescTable * desc = table + index;
		
		// do we have a bubble to draw?
		if ( desc->descBubbleCounter > 0 )
			{
			// remember we need to draw it
			drawTable[ index ] = 1;
			
			// save location
			desc->descBubbleLoc.Set( dsm->dsmHorz, dsm->dsmVert );
			desc->descMobile = dsm;
			}
		
		++dsm;
		}
	
	// decrement the bubble counts in all of the descriptors
	int skipCounter = 0;
	for(;;)
		{
		// find the bubble with the oldest counter
		int oldestIndex = -1;
		int oldestCounter = INT_MAX;
		table = gDescTable;
		for ( int index = 0;  index < kDescTableSize;  ++index, ++table )
			{
			int counter = table->descBubbleCounter;
			if ( counter <= skipCounter )
				continue;
			if ( counter < oldestCounter )
				{
				oldestCounter = counter;
				oldestIndex   = index;
				}
			}
		if ( oldestIndex < 0 )
			break;
		skipCounter = oldestCounter - 1;
		
		// decrement the counter
		table = gDescTable + oldestIndex;
		table->descBubbleCounter = oldestCounter - 1;
		
		// draw all onscreen mobiles
		if ( drawTable[ oldestIndex ] )
			{
			Draw1Bubble( table, kDrawBubbleNormal );
			continue;
			}
		
		// draw all thoughts
		int bType = table->descBubbleType;
		if ( kBubbleThought == bType )
			{
			Draw1Bubble( table, kDrawBubbleNormal );
			continue;
			}
		
		// draw offscreen yellers
		if ( bType & kBubbleFar )
			{
			if ( kBubbleFar + kBubbleRealAction == bType
			||   kBubbleFar + kBubblePlayerAction == bType )
				{
				Draw1Bubble( table, kDrawBubbleNormal );
				}
			else
				Draw1Bubble( table, kDrawBubbleNamePrefix );
			continue;
			}
		
		// draw yellers that are onscreen but not in the mobile list
		if ( kBubbleYell == bType )
			{
			if ( kBubblePosNone == table->descBubblePos )
				{
				table->descBubbleLoc.ptV = gLayout.layoFieldBox.rectTop
										 - gFieldCenter.ptV;
				table->descBubbleLoc.ptH = gLayout.layoFieldBox.rectLeft
										 - gFieldCenter.ptH;
				}
			Draw1Bubble( table, kDrawBubbleNamePrefix );
			continue;
			}
		}
}


/*
**	AdjustBubbleQD()
**
**	rearrange a text bubble's contents, via QuickDraw
*/
static void
AdjustBubbleQD( DTSImage * pix, const DTSRect * box )
{
	int width = box->rectRight - box->rectLeft;
	int height = box->rectBottom - box->rectTop;
	
	GWorldPtr bitWorld;
	OSStatus err = NewGWorld( &bitWorld, 8, DTS2Mac(box), 0, 0, 0 );
	if ( noErr == err )
		{
		PixMapHandle pm = GetGWorldPixMap( bitWorld );
		if ( LockPixels( pm ) )
			{
			int srcRowBytes = pix->GetRowBytes();
			const uchar * src = static_cast<uchar *>( pix->GetBits() );
			
			int dstRowBytes = GetPixRowBytes( pm ) & 0x3FFF;
			uchar * dst = reinterpret_cast<uchar *>( GetPixBaseAddr( pm ) );
			
			// advance src to first row...
			src += srcRowBytes * box->rectTop;
			// ... and thence to first column
			src += box->rectLeft;
			
			// advance dst to last row...
			dst += dstRowBytes * (height - 1);
			// ... thence to last pixel
			dst += width - 1;
			
			// inter-row gaps
			srcRowBytes -= width;
			dstRowBytes -= width;
			
			for ( int row = height; row > 0 ; --row )
				{
				// copy this row
				for ( int column = width; column > 0; --column )
					{
					// copy this pixel
					*dst = *src;
					--dst;
					++src;
					}
				
				src += srcRowBytes;
				dst -= dstRowBytes;
				}
			
			const BitMap * bm = GetPortBitMapForCopyBits( bitWorld );
			CopyBits( bm, DTS2Mac(pix), DTS2Mac(box), DTS2Mac(box), srcCopy, 0 );
			UnlockPixels( pm );
			}
		DisposeGWorld( bitWorld );
		}
}


#ifdef USE_OPENGL
/*
**	AdjustBubbleGL()
**	OpenGL flavor of the above
**	kind of a dumb way to do it, but it's better than nothing at all
*/
static void NOT_USED
AdjustBubbleGL( const DTSRect * box )
{
#if TARGET_CPU_PPC
	// dunno why this is needed, but on my creaky old PowerBook G4 (OS 10.5),
	// it is.
	(const_cast<DTSRect *>( box ))->rectLeft -= 1;
#endif
	
	int width = box->rectRight - box->rectLeft;
	int height = box->rectBottom - box->rectTop;
	int rb = (4 * width + 3) & ~3;	// 32 bpp; round up to 4-byte alignment
	UInt32 * buf = NEW_TAG("LudeBits") UInt32[ height * (rb / 4) ];
	if ( not buf )
		return;
//	memset( buf, 0, height * rb );
	
	// Is this right? what about dimmed or non-opaque bubbles? Or have they
	// already been properly handled, prior to this stage?
	disableTexturing();
	disableBlending();
	disableAlphaTest();
	
	// copy existing bubble bits into buffer
	int fieldHeight = gLayout.layoFieldBox.rectBottom
					- gLayout.layoFieldBox.rectTop;
	glReadPixels( box->rectLeft - gLayout.layoFieldBox.rectLeft,
				  fieldHeight - box->rectBottom + kTextAscent - 1,
				  width, height, GL_BGRA, GL_UNSIGNED_INT_8_8_8_8_REV, buf );
	
	// src points to first pixel
	UInt32 * src = buf;
	
	// dst points to one-after-last pixel
	UInt32 * dst = buf + (height - 1) * (rb / 4) + width;
	
	// swap buffer end-for-end
	for ( int row = height - 1; row >= 0; --row )
		{
		// we can stop swapping as soon as the src and dst pointers
		// overtake each other
		if ( src > dst )
			break;
		
		for ( int column = width - 1; column >= 0; --column )
			{
			if ( src > dst )	// ditto
				break;
			
			// interchange, fore and aft
			UInt32 pixel = *src;
			*src++ = *--dst;
			*dst = pixel;
			}
		
		// offset to next/prev row
		src += rb / 4 - width;
		dst -= rb / 4 - width;
		}
	
	// blast swizzled pix back into bubble
	glRasterPos2i( gFieldCenter.ptH, gFieldCenter.ptV );
	glBitmap( 0, 0, 0, 0,
		box->rectLeft - gFieldCenter.ptH + 0.5f,
		-(box->rectBottom - gFieldCenter.ptV + 0.5f),
		nullptr );
	
	glDrawPixels( width, height, GL_BGRA, GL_UNSIGNED_INT_8_8_8_8_REV, buf );
	
	delete[] buf;
}
#endif  // USE_OPENGL


/*
**	AdjustBubble()
**	rearrange a text bubble's content, for purposes of ludification
*/
static void
AdjustBubble( DTSImage * pix, const DTSRect * box )
{
	// GL and QD use totally different methods
#ifdef USE_OPENGL
	if ( gUsingOpenGL )
		AdjustBubbleGL( box );
	else
#endif
		AdjustBubbleQD( pix, box );
}


/*
**	IsLudic()
**	Is ludification appropriate, either in general
**	or for this specific descriptor?
*/
static bool
IsLudic( const DescTable * desc )
{
	// for all player bubbles?
	if ( gFlipAllBubbles
	&&   ( kDescPlayer == desc->descType /* || kDescNPC == desc->descType */ ) )
		{
		return true;
		}
	
	// for a certain very specific circumstance?
	if ( kDescPlayer == desc->descType
	&&	 desc->descMobile
	&&	 (    ( desc->descID == 1261 || desc->descID == 1262 )
		   && ( desc->descMobile->dsmState == 7 ||
		   		desc->descMobile->dsmState == 11 ) ) )
		{
		return true;
		}
	
	return false;
}


/*
**	CLOffView::Draw1Bubble()
**
**	Draw one bubble of text
*/
void
CLOffView::Draw1Bubble( DescTable * desc, int mode )
{
	int friends = kFriendNone;
	
	// avoid the tree lookup unless the bubble is from a player, or is a thought
	if ( kDescPlayer == desc->descType
	||   kDescThoughtBubble == desc->descType )
		{
		friends = GetPlayerIsFriend( desc );
		}
	
	// stop now if we're ignoring this player
	if ( kFriendBlock == friends
	||	 kFriendIgnore == friends )
		{
		return;
		}
	
	// draw "me" as a friend  (unless you've invoked TedMode(tm))
	if ( gThisPlayer == desc
	&&	 not gSpecialTedFriendMode )
		{
		friends = kFriendLabel1;
		}
	
	// munge the text
	// prefix the descriptor's name in front
	char buff[ kMaxDescTextLength + kMaxNameLen + 4 ];
	const char * text = desc->descBubbleText;
	if ( kDrawBubbleNamePrefix == mode )
		{
		snprintf( buff, sizeof buff, "%s: %s", desc->descName, text );
		text = buff;
		}
	
#ifdef USE_OPENGL
// we need to use gCharWidths, which is initialized in TextView::DoDraw()
// (why not in an initializer?) -- A. because GetTextWidth() requires
//	a valid QDPort, which we cannot count on having until DoDraw() time.
	if ( not gUsingOpenGL
	||   0 == gCharWidths[ (uint) 'A' ] )
#endif	// USE_OPENGL
		{
		// draw the string into the offscreen text view
		offTextView.SetText( text, kFriendNone != friends );
		offTextView.Draw();
		}
	
	// calculate where the words start and end
	WordTable * words = GetWordTable( text );
	
	// adjust the word table if there are words longer than one line
	words = AdjustWordTable( words );
	
	// determine which bubble image to use
	DTSImage * image;
	WordTable lines[4];
#ifdef USE_OPENGL
	ImageCache * cache = nullptr;
#endif	// USE_OPENGL
	
	int interiorWidth = kTextSmallWidth;
	int numLines = InitLineTable( lines, words, kTextSmallWidth );
	if ( numLines <= 2 )
		{
		image = &offSmallBubbleImage->icImage.cliImage;
		
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			cache = offSmallBubbleImage;
#endif	// USE_OPENGL
		}
	else
		{
		numLines = InitLineTable( lines, words, kTextMediumWidth );
		if ( numLines <= 3 )
			{
			image = &offMediumBubbleImage->icImage.cliImage;
			interiorWidth = kTextMediumWidth;
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				cache = offMediumBubbleImage;
#endif	// USE_OPENGL
			}
		else
			{
			numLines = InitLineTable( lines, words, kTextLargeWidth );
			image = &offLargeBubbleImage->icImage.cliImage;
			interiorWidth = kTextLargeWidth;
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				cache = offLargeBubbleImage;
#endif	// USE_OPENGL
			}
		}
	
	// choose a position if we haven't yet
	if ( kBubblePosNone == desc->descBubblePos )
		{
		if ( kBubbleThought == desc->descBubbleType )
			ChooseThoughtPosition( image, desc );
		else
			ChooseBubblePosition( image, desc );
		}
	
	// find a good position for the bubble
	int pos = desc->descBubblePos;
	PositionBubble( &desc->descBubbleBox, image, desc, pos );
	pos = PinBubble( &desc->descBubbleBox, pos );
	desc->descBubbleLastPos = pos;
	
	// calculate the source rect
	DTSRect src;
	int bType = desc->descBubbleType;
	if ( bType & kBubbleNotCommon )
		{
		int code = desc->descBubbleLanguage & kBubbleCodeMask;
		if ( code != kBubbleCodeKnown )
			{
#if USE_WHISPER_YELL_LANGUAGE
			if ( (bType & kBubbleTypeMask) != kBubbleYell )
				bType = kBubbleRealAction;
#else
			bType = kBubbleRealAction;
#endif	// USE_WHISPER_YELL_LANGUAGE
			}
		}
	
	int bubblemap = gBubbleMap[ bType & kBubbleTypeMask ];
	
	// special handling for action (red/blue/green) square bubbles:
	// determine which row to use for this type
	if ( kBubbleRealAction == bubblemap )
		{
		bubblemap = kBubbleRealAction;
		switch ( bType & kBubbleTypeMask )
			{
			case kBubbleRealAction:
				// blue border - top row
				pos = kBubblePosUpperLeft;
				break;
			
			case kBubblePlayerAction:
				// red border - second row
				pos = kBubblePosUpperRight;
				break;
			
			case kBubbleNarrate:
				// green border - third row
				pos = kBubblePosLowerRight;
				break;
			
			default:
				ShowMessage( "unknown bubbleType %d", bType & kBubbleTypeMask );
				break;
			}
		}
	
	DTSRect imageBounds;
	image->GetBounds( &imageBounds );
	int height    = imageBounds.rectBottom / kNumBubblePositions;
	int top       = height * ( pos - kBubblePosUpperLeft );
	int width     = imageBounds.rectRight / kNumBubbleTypes;
	int left      = bubblemap * width;
	src.rectTop    = top;
	src.rectLeft   = left;
	src.rectBottom = top  + height;
	src.rectRight  = left + width;
	
	// draw the bubble using preferred opacity
#ifdef USE_OPENGL
	GLfloat targetAlphaScale = 1.0f;	// this means use the alpha
										// implied in the alpha map
	if ( gUsingOpenGL )
		{
# ifdef OGL_USE_TEXT_FADE
		if ( desc->descBubbleCounter <= kTextFadeLength )
			{
			if ( kBlitterTransparent == gPrefsData.pdFriendBubbleBlitter
			||   kBlitterTransparent == gPrefsData.pdBubbleBlitter )
				{
				// would be better if this check were farther up,
				// but don't want to disturb the rest of the code too much
				
				desc->descBubbleCounter -= kTextFadeLength;	// or should this be = 0?
				return;
				}
			else
				{
				const GLfloat invFadeLengthPlusOne = 1.0f / (kTextFadeLength + 1);
				targetAlphaScale =
					(desc->descBubbleCounter + 1) * invFadeLengthPlusOne;
				}
			}
# endif	// OGL_USE_TEXT_FADE
		
		int blitter;
		if ( friends
		||	kBubbleThought == desc->descBubbleType
		||	desc->descType != kDescPlayer )
			{
			blitter = gPrefsData.pdFriendBubbleBlitter;
			}
		else
			// translucent for everything else
			blitter = gPrefsData.pdBubbleBlitter;
		
		switch ( blitter )
			{
			case kBlitter75:
				drawOGLPixmapAsTexture(	src, desc->descBubbleBox,
					cache->icImage.cliImage.GetRowBytes(),
					kIndexToAlphaMapAlpha25TransparentZero,
					TextureObject::kBalloonArray, cache, targetAlphaScale );
				break;
			
			case kBlitter50:
				drawOGLPixmapAsTexture(	src, desc->descBubbleBox,
					cache->icImage.cliImage.GetRowBytes(),
					kIndexToAlphaMapAlpha50TransparentZero,
					TextureObject::kBalloonArray, cache, targetAlphaScale );
				break;
			
			case kBlitter25:
				drawOGLPixmapAsTexture(	src, desc->descBubbleBox,
					cache->icImage.cliImage.GetRowBytes(),
					kIndexToAlphaMapAlpha75TransparentZero,
					TextureObject::kBalloonArray, cache, targetAlphaScale );
				break;
			
			case kBlitterTransparent:
				drawOGLPixmapAsTexture(	src, desc->descBubbleBox,
					cache->icImage.cliImage.GetRowBytes(),
					kIndexToAlphaMapAlpha100TransparentZero,
					TextureObject::kBalloonArray, cache, targetAlphaScale );
				break;
			}
		}
	else
#endif	// USE_OPENGL
		{
		if ( friends
		||	kBubbleThought == desc->descBubbleType
		||	desc->descType != kDescPlayer )
			{
			SelectiveBlit( this, image, &src,
				&desc->descBubbleBox, gPrefsData.pdFriendBubbleBlitter );
			}
		else
			// translucent for everything else
			SelectiveBlit( this, image, &src,
				&desc->descBubbleBox, gPrefsData.pdBubbleBlitter );
		}
	
	// draw the lines
	WordTable * line = lines;
	src.rectTop    = 0;
	src.rectBottom = kTextAscent + kTextDescent;
	DTSRect dst    = desc->descBubbleBox;
	dst.rectTop   += kTextInsetVert;
	int midline    = dst.rectLeft + dst.rectRight;
	
	// medium-sized talk bubble images are one or two pixels off-center
	if ( kTextMediumWidth == interiorWidth
	&&	 kBubbleRealAction   != bubblemap
	&&	 kBubblePlayerAction != bubblemap )
		{
		if ( kBubblePosUpperRight == pos || kBubblePosLowerRight == pos )
			midline += 3;
		else
			--midline;
		}
	
	// initialize the bounding box accumulator
	DTSRect dst2;
	dst2.Set( 1024, dst.rectTop, -1024, dst.rectBottom - kTextInsetVert );
	
	for ( ;  numLines > 0;  --numLines )
		{
		dst.rectBottom = dst.rectTop + kTextAscent + kTextDescent;
		int start = line->startPos;
		int stop  = line->stopPos;
		if ( kTextViewWidth < start )
			start = kTextViewWidth;
		if ( kTextViewWidth < stop )
			stop = kTextViewWidth;
		
		// tack on one extra pixel if the line ends with 'f'
		if ( line->pad )
			stop += 1;
		
		int width = stop - start;
		
		src.rectLeft   = start;
		src.rectRight  = stop;
		dst.rectLeft   = (midline - width) / 2;
		dst.rectRight  = dst.rectLeft + width;
		
		// grow the bounding box
		if ( dst.rectLeft < dst2.rectLeft )
			dst2.rectLeft = dst.rectLeft;
		if ( dst.rectRight > dst2.rectRight )
			dst2.rectRight = dst.rectRight;
		
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
				// this should be moved up...
			disableAlphaTest();
			disableTexturing();
			
# ifdef OGL_USE_TEXT_FADE
			if ( desc->descBubbleCounter <= kTextFadeLength
			&&   targetAlphaScale < 1.0f )
				{
				enableBlending();
				glColor4f( 0.0f, 0.0f, 0.0f, targetAlphaScale );
				}
			else
				{
				// faster than bothering with the index to alpha stuff
				disableBlending();
				glColor3us( 0x0000, 0x0000, 0x0000 );
				}
# else
			// faster than bothering with the index to alpha stuff
			disableBlending();
			glColor3us( 0x0000, 0x0000, 0x0000 );
# endif	// OGL_USE_TEXT_FADE
			
			int pixelPosition = 0;
			int charStartPosition = 0;
			if ( text[ 0 ] )	// this is to fix the "option-space bug"
				{
				while( line->startPos > pixelPosition )
					{
					pixelPosition += gCharWidths[
						*reinterpret_cast<const uchar *>( text + charStartPosition ) ];
					++charStartPosition;
					}
				int charStopPosition = charStartPosition;
				do {
					pixelPosition += gCharWidths[
						*reinterpret_cast<const uchar *>( text + charStopPosition ) ];
					++charStopPosition;
					} while( line->stopPos > pixelPosition - 1 );
				
				drawOGLText( dst.rectLeft, dst.rectTop + kTextAscent,
					geneva9NormalListBase,
					&text[charStartPosition], charStopPosition - charStartPosition );
				}
			}
		else
#endif	// USE_OPENGL
			BlitTransparent( this, offTextView.GetTVImage(), &src, &dst );
		
		dst.rectTop += kTextLineHeight;
		++line;
		}
	
	// do special ludified cases
	if ( IsLudic( desc ) )
		AdjustBubble( offImage, &dst2 );
	
	// is it a non-Common language? if so, draw it special somehow
#if USE_WHISPER_YELL_LANGUAGE
	if ( (bType & kBubbleNotCommon)
	&&	 (desc->descBubbleLanguage & kBubbleCodeMask) == kBubbleCodeKnown )
#else
	if ( bType & kBubbleNotCommon )
#endif	// USE_WHISPER_YELL_LANGUAGE
		{
		uint bubColor = desc->descBubbleLanguage & kBubbleLanguageMask;
		
		// map "common" to no color
		if ( kBubbleCommon != bubColor
		&&	 bubColor < sizeof(sLangColors)/sizeof(RGBColor) )
			{
			// ** how handle the bright-colors pref?
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
# ifdef OGL_USE_TEXT_FADE
				if ( desc->descBubbleCounter <= kTextFadeLength
				&&   targetAlphaScale < 1.0f )
					{
					// blending was already enabled above
					glColor4us( sLangColors[bubColor].red,
						sLangColors[bubColor].green,
						sLangColors[bubColor].blue,
						targetAlphaScale * 0xFFFF );
					}
				else
					glColor3us( sLangColors[bubColor].red,
								sLangColors[bubColor].green,
								sLangColors[bubColor].blue );
# else
				glColor3us( sLangColors[bubColor].red,
							sLangColors[bubColor].green,
							sLangColors[bubColor].blue );
# endif	// OGL_USE_TEXT_FADE
			else
#endif	// USE_OPENGL
				::RGBForeColor( &sLangColors[bubColor] );
			
			dst2.rectLeft = (midline - interiorWidth) / 2;
			
			// adapt to a 1-pixel positioning error in the talk balloon images.
			// only the right-hand large whisper balloons seem to have this problem,
			// and it is in both renderers (QD and GL)
			// [Why don't we just correct the bubble image itself? It's image #3,
			// second column, rows 2 & 3. Then all this can go.  Except, hmmm...
			// does the windows client perform the same fixup?]
			
			if (
				((bType & kBubbleTypeMask) == kBubbleWhisper )
				 	&&
				( kTextLargeWidth == interiorWidth )
				 	&&
				(
					// should I use pos instead?
					( kBubblePosUpperRight == desc->descBubblePos )	
						||
					( kBubblePosLowerRight == desc->descBubblePos )
				)
			)
				{
				dst2.rectLeft -= 1;
				}
			
			dst2.rectRight = dst2.rectLeft + interiorWidth;
			dst2.Inset( -3, 0 );
			
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				{
				glLineWidth( 2.0 );
				glBegin( GL_LINES );	// individual line segments, corners unconnected
				
			// line width 2 implies the center is ON A GRID LINE (so no + 0.5f)
			// which means we can use shorts instead of floats
			// (like that's a big deal anymore...)
			
					glVertex2s( dst2.rectLeft + 1, dst2.rectTop + 4 );
					glVertex2s( dst2.rectLeft + 1, dst2.rectBottom - 4 );
					
					glVertex2s( dst2.rectRight - 4, dst2.rectTop + 1 );
					glVertex2s( dst2.rectLeft + 4, dst2.rectTop + 1 );
					
					glVertex2s( dst2.rectRight - 1, dst2.rectTop + 4 );
					glVertex2s( dst2.rectRight - 1, dst2.rectBottom - 4 );
					
					glVertex2s( dst2.rectRight - 4, dst2.rectBottom - 1 );
					glVertex2s( dst2.rectLeft + 4, dst2.rectBottom - 1 );
				glEnd();
				
			// the round part of the round cornered rectangle
				glPixelStorei( GL_UNPACK_ALIGNMENT, 1 );
				static const uchar corner[] =
					{
					0x00, 0x3c, 0x7e, 0x66,
					0x66, 0x7e, 0x3c, 0x00
					};
				
				// note that GL_UNPACK_ROW_LENGTH is not used here,
				// the default row length for bitmaps is 8, not 'width'
				
				// guaranteed valid
				glRasterPos2i( gFieldCenter.ptH, gFieldCenter.ptV );
				
				glBitmap( 0, 0, 0, 0,
					dst2.rectLeft - gFieldCenter.ptH + 4 + 0.5f,
					-(dst2.rectBottom - gFieldCenter.ptV - 5 + 0.5f),
										// -v cuz of glScale(1, -1)
					nullptr );
				
					// lower left
				glBitmap( 4, 4, 4, 4, 0,
					dst2.rectBottom - dst2.rectTop - 4, corner );	// ditto
				
					// upper left
				glPixelStorei( GL_UNPACK_SKIP_ROWS, 4 );
				glBitmap( 4, 4, 4, 4,
					dst2.rectRight - dst2.rectLeft - 4 , 0, corner );	// ditto
				
					// upper right
				glPixelStorei( GL_UNPACK_SKIP_PIXELS, 4 );
				glBitmap( 4, 4, 4, 4, 0,
					- (dst2.rectBottom - dst2.rectTop - 4), corner );	// -v
				
					// lower right
				glPixelStorei( GL_UNPACK_SKIP_ROWS, 0 );
				glBitmap( 4, 4, 4, 4, 0, 0, corner );	// -v cuz of glScale(1, -1)
				
				// restore whatever we changed
				// GL_UNPACK_SKIP_ROWS was already reset
				glPixelStorei( GL_UNPACK_SKIP_PIXELS, 0 );
				glPixelStorei( GL_UNPACK_ALIGNMENT, 4 );
				}
			else
#endif	// USE_OPENGL
				{
				DTSRound innards;
				innards.Set( dst2.rectLeft, dst2.rectTop,
							 dst2.rectRight, dst2.rectBottom );
				innards.SetRound( 12, 12 );
				SetPenSize( 2 );
				Frame( &innards );
				}
			
#ifdef USE_OPENGL
			if ( gUsingOpenGL )
				{
				glColor3us( 0, 0, 0 );
				glLineWidth(1.0);
				}
			else
#endif	// USE_OPENGL
				{
				SetForeColor( &gBackColorTable[kColorCodeBackBlack] );	// always 0s
				SetPenSize( 1 );
				}
			}
		}
	
	// dispose of the word table
	delete[] words;
}


/*
**	GetWordTable()
**
**	allocate and initialize an array that defines where words start and end
*/
WordTable *
GetWordTable( const char * text )
{
	// count the words
	// add one so they know where to stop
	int numWords = 1;
	uchar ch;
	const uchar * ptr = reinterpret_cast<const uchar *>( text );
	int increment = 1;
	for(;;)
		{
		ch = *ptr++;
		if ( ch == ' '
		||   ch == '\t'
		||   ch == '\n'
		||   ch == '\r' )
			{
			increment = 1;  // 'ch' is a word-separator
			}
		else
			{
			numWords += increment;
			increment = 0;
			if ( '\0' == ch )
				break;
			}
		}
	
	// allocate the word table
	WordTable * words = NEW_TAG("WordTable") WordTable[ numWords ];
	if ( not words )
		return nullptr;
	
	// fill in the word table
	ptr = reinterpret_cast<const uchar *>( text );
	int pos = 0;
	increment = 1;
	WordTable * word = words;
	int index = 0;
	for(;;)
		{
		ch = *ptr;
		if ( ch == ' '
		||   ch == '\t'
		||   ch == '\n'
		||   ch == '\r'
		||   ch == '\0' )
			{
			if ( 0 == increment )
				{
				word->stopPos = pos - 1;
				word->length  = ptr - reinterpret_cast<const uchar *>( word->text );
				// silly hack for the overhanging 'f' problem
				word->pad	  = (ptr[ -1] == 'f');
				++word;
				}
			if ( '\0' == ch )
				break;
			increment = 1;
			}
		else
			{
			if ( increment )
				{
				word->startPos = pos;
				word->text     = reinterpret_cast<const char *>( ptr );
				}
			increment = 0;
			}
		++ptr;
		++index;
		pos += gCharWidths[ ch ];
		}
	
	// fill in the terminator
	word->startPos = -1;
	
	return words;
}


/*
**	AdjustWordTable()
**
**	handle words that are just plain too long
**	basically we break the too-long words up into
**	small words each one character long
*/
WordTable *
AdjustWordTable( WordTable * words )
{
	// first check and see if there is a word that is too long
	WordTable * src;
	bool bTooLong = false;
	int width;
	for ( src = words;  src->startPos >= 0;  ++src )
		{
		width = src->stopPos - src->startPos;
		if ( width > kTextLargeWidth )
			{
			bTooLong = true;
			break;
			}
		}
	// we don't have to make any changes
	if ( not bTooLong )
		return words;
	
	// count the new number of word table entries
	int count = 1;
	for ( src = words;  src->startPos >= 0;  ++src )
		{
		width = src->stopPos - src->startPos;
		if ( width > kTextLargeWidth )
			count += src->length;
		else
			++count;
		}
	
	// allocate a new word table
	WordTable * newwords = NEW_TAG("WordTable") WordTable[ count ];
	if ( not newwords )
		{
		// oh dear - we're kinda screwed
		// do the best we can
		// this should show an empty bubble
		words->startPos = -1;
		return words;
		}
	
	// fill in the new word table
	WordTable * dst = newwords;
	for ( src = words;  src->startPos >= 0;  ++src )
		{
		width = src->stopPos - src->startPos;
		if ( width > kTextLargeWidth )
			{
			// break up this word into single-character words
			const char * text = src->text;
			int pos = src->startPos;
			for ( count = src->length;  count > 0;  --count )
				{
				dst->startPos = pos;
				pos += gCharWidths[ * reinterpret_cast<const uchar *>( text ) ];
				dst->stopPos  = pos - 1;
				dst->text     = text;
				dst->length   = 1;
				
				// single-letter words that are 'f's get padded too
				dst->pad	  = ( *text == 'f' );
				++dst;
				++text;
				}
			}
		else
			*dst++ = *src;
		}
	dst->startPos = -1;
	
	// dispose of the old word table and return the new
	delete[] words;
	
	return newwords;
}


/*
**	InitLineTable()
**
**	initialize the line table
*/
int
InitLineTable( WordTable * lines, WordTable * words, int width )
{
	int lineCount = 1;
	for(;;)
		{
		words = Init1Line( lines, words, width );
		if ( words->startPos < 0 )
			break;
		if ( lineCount >= 4 )
			break;
		++lineCount;
		++lines;
		}
	
	return lineCount;
}


/*
**	Init1Line()
**
**	initialize one line table
*/
WordTable *
Init1Line( WordTable * line, WordTable * words, int width )
{
	line->startPos = words->startPos;
	line->stopPos  = words->stopPos;
	
	int limit = width + line->startPos;
	for(;;)
		{
		if ( words->startPos < 0 )
			return words;
		if ( words->stopPos > limit )
			return words;
		line->stopPos = words->stopPos;
		
		// if the word ends with 'f' then so does the line
		line->pad = words->pad;
		++words;
		}
}


/*
**	PositionBubble()
**
**	position the bubble
*/
void
PositionBubble( DTSRect * dst, const DTSImage * image,
				const DescTable * desc, int pos )
{
	int horz = desc->descBubbleLoc.ptH;
	int vert = desc->descBubbleLoc.ptV;
	
	int top, left, bottom, right;
	
	// calculate the destination rect
	DTSRect imageBounds;
	image->GetBounds( &imageBounds );
	int size = imageBounds.rectBottom / kNumBubblePositions;
	if ( kBubblePosUpperLeft == pos
	||   kBubblePosUpperRight == pos )
		{
		bottom = gFieldCenter.ptV + vert - kTextUpperOffset;
		top    = bottom - size;
		}
	else
		{
		top    = gFieldCenter.ptV + vert + kTextLowerOffset;
		bottom = top + size;
		}
	size = imageBounds.rectRight / kNumBubbleTypes;
	if ( kBubblePosUpperLeft == pos
	||   kBubblePosLowerLeft == pos )
		{
		right = gFieldCenter.ptH + horz - kTextLeftOffset;
		left  = right - size;
		}
	else
		{
		left  = gFieldCenter.ptH + horz + kTextRightOffset;
		right = left + size;
		}
	
	dst->rectTop    = top;
	dst->rectLeft   = left;
	dst->rectBottom = bottom;
	dst->rectRight  = right;
}


/*
**	PinBubble()
**
**	pin the bubble to be onscreen
**	change the position if necessary
*/
int
PinBubble( DTSRect * dst, int pos )
{
	int top    = dst->rectTop;
	int left   = dst->rectLeft;
	int bottom = dst->rectBottom;
	int right  = dst->rectRight;
	
	int test = gLayout.layoFieldBox.rectTop - kBubblePadVert;
	if ( top < test )
		{
		bottom = test + bottom - top;
		top    = test;
		if ( pos == kBubblePosUpperLeft )
			pos = kBubblePosLowerLeft;
		else
		if ( pos == kBubblePosUpperRight )
			pos = kBubblePosLowerRight;
		}
	test = gLayout.layoFieldBox.rectBottom + kBubblePadVert;
	if ( bottom > test )
		{
		top    = test - ( bottom - top );
		bottom = test;
		if ( pos == kBubblePosLowerLeft )
			pos = kBubblePosUpperLeft;
		else
		if ( pos == kBubblePosLowerRight )
			pos = kBubblePosUpperRight;
		}
	test = gLayout.layoFieldBox.rectLeft - kBubblePadHorz;
	if ( left < test )
		{
		right = test + right - left;
		left  = test;
		if ( pos == kBubblePosUpperLeft )
			pos = kBubblePosUpperRight;
		else
		if ( pos == kBubblePosLowerLeft )
			pos = kBubblePosLowerRight;
		}
	test = gLayout.layoFieldBox.rectRight + kBubblePadHorz;
	if ( right > test )
		{
		left  = test - ( right - left );
		right = test;
		if ( pos == kBubblePosLowerRight )
			pos = kBubblePosLowerLeft;
		else
		if ( pos == kBubblePosUpperRight )
			pos = kBubblePosUpperLeft;
		}
	
	dst->rectTop    = top;
	dst->rectLeft   = left;
	dst->rectBottom = bottom;
	dst->rectRight  = right;
	
	return pos;
}


/*
**	ChooseBubblePosition()
**
**	choose a position for the bubble
*/
void
ChooseBubblePosition( const DTSImage * image, DescTable * desc )
{
	// calculate the values for the bubble at each position
	int ul = CalcBubblePosValue( image, desc, kBubblePosUpperLeft  );
	int ur = CalcBubblePosValue( image, desc, kBubblePosUpperRight );
	int ll = CalcBubblePosValue( image, desc, kBubblePosLowerLeft  );
	int lr = CalcBubblePosValue( image, desc, kBubblePosLowerRight );
	
	// modify the values based on the direction the mobile is facing
	int mobFacing = desc->descMobile
				  ? desc->descMobile->dsmState / 4 : kFacingSouth;
	switch ( mobFacing )
		{
		case kFacingEast:		// east
			ul += 1;
			ll += 1;
			break;
		
		case kFacingSE:			// southeast
			ul += 2;
			ur += 1;
			ll += 1;
			break;
		
		case kFacingSouth:		// south
		default:				// also poses 32-47
			ul += 1;
			ur += 1;
			break;
		
		case kFacingSW:			// southwest
			ul += 1;
			ur += 2;
			lr += 1;
			break;
		
		case kFacingWest:		// west
			ur += 1;
			lr += 1;
			break;
		
		case kFacingNW:			// northwest
			ur += 1;
			lr += 2;
			ll += 1;
			break;
		
		case kFacingNorth:		// north
			ll += 1;
			lr += 1;
			break;
		
		case kFacingNE:			// northeast
			ul += 1;
			ll += 2;
			lr += 1;
			break;
		}
	
	// choose the position with the highest score
	int pos = kBubblePosUpperLeft;
	if ( ul < ur )
		{
		pos = kBubblePosUpperRight;
		ul = ur;
		}
	int pos1 = kBubblePosLowerLeft;
	if ( ll < lr )
		{
		pos1 = kBubblePosLowerRight;
		ll = lr;
		}
	if ( ul < ll )
		pos = pos1;
	desc->descBubblePos = pos;
}


/*
**	CalcBubblePosValue()
**
**	calculate the value of the bubble in this position
**	-1000 points for going off the screen
**	  -20 points for every other bubble you cover
**	  -10 points for every mobile you cover
**	   +1 points for being behind a character facing n s e w
**	   +1 points for being beside a character facing ne nw se sw
**	   +2 points for being behind a character facing ne nw se sw
**	   +3 points for being in the same place as the last bubble
*/
int
CalcBubblePosValue( const DTSImage * image, DescTable * desc, int pos )
{
	// position the bubble
	DTSRect bounds;
	PositionBubble( &bounds, image, desc, pos );
	
	// shrink it a bit
	bounds.rectTop    += 3;
	bounds.rectLeft   += 8;
	bounds.rectBottom -= 4;
	bounds.rectRight  -= 8;
	
	// check if off the screen
	int value = 0;
	if ( bounds.rectTop    < gLayout.layoFieldBox.rectTop
	||   bounds.rectLeft   < gLayout.layoFieldBox.rectLeft
	||   bounds.rectBottom > gLayout.layoFieldBox.rectBottom
	||   bounds.rectRight  > gLayout.layoFieldBox.rectRight )
		{
		value -= 1000;
		}
	
	// check mobiles that are covered
	const DSMobile * dsm = gDSMobile;
	DTSRect sect;
	for ( int count = gNumMobiles;  count > 0;  --count, ++dsm )
		{
		DescTable * otherdesc = gDescTable + dsm->dsmIndex;
		if ( otherdesc == desc )
			continue;
		
		ImageCache * ic = CachePicture( otherdesc );
		if ( not ic )
			{
			DTSPoint loc;
			loc.Set( dsm->dsmHorz + gFieldCenter.ptH,
					 dsm->dsmVert + gFieldCenter.ptV );
			if ( loc.InRect( &bounds ) )
				value -= 10;
			}
		else
			{
			int mobileSize = ic->icBox.rectRight / 32;
			int halfSize   = mobileSize / 2;
			sect.rectTop    = dsm->dsmVert + gFieldCenter.ptV - halfSize;
			sect.rectLeft   = dsm->dsmHorz + gFieldCenter.ptH - halfSize;
			sect.rectBottom = sect.rectTop  + mobileSize;
			sect.rectRight  = sect.rectLeft + mobileSize;
			sect.Intersect( &bounds );
			if ( not sect.IsEmpty() )
				value -= 10;
			}
		}
	
	// check other bubbles
	const DescTable * table = gDescTable;
	for ( int index = kDescTableSize;  index > 0;  --index, ++table )
		{
		if ( table->descBubbleCounter <= 0 )
			continue;
		int testpos = table->descBubblePos;
		if ( kBubblePosNone == testpos )
			continue;
		sect = bounds;
		sect.Intersect( &table->descBubbleBox );
		if ( sect.IsEmpty() )
			continue;
		value -= 20;
		}
	
	// check the position of the last drawn bubble
	if ( pos == desc->descBubbleLastPos )
		value += 3;
	
	return value;
}


/*
**	ChooseThoughtPosition()
**
**	choose a position for a thought bubble
*/
void
ChooseThoughtPosition( const DTSImage * /* image */, DescTable * desc )
{
	int index = ( desc - gDescTable ) - kDescTableBaseSize;
	int bpr = (gLayout.layoFieldBox.rectRight - gLayout.layoFieldBox.rectLeft)
			/ kBubbleLargeWidth;
	int row = index / bpr;
	int col = index % bpr;
	int flg = row & 1;
	row >>= 1;
	
	if ( flg )
		{
		// build from the bottom up
		desc->descBubblePos     = kBubblePosUpperRight;
		desc->descBubbleLoc.ptV = gLayout.layoFieldBox.rectBottom
				- gFieldCenter.ptV - row * kBubbleLargeHeight;
		desc->descBubbleLoc.ptH = gLayout.layoFieldBox.rectLeft
				- gFieldCenter.ptH + col * kBubbleLargeWidth;
		}
	else
		{
		// build from the top down
		desc->descBubblePos     = kBubblePosLowerRight;
		desc->descBubbleLoc.ptV = gLayout.layoFieldBox.rectTop
				- gFieldCenter.ptV + row * kBubbleLargeHeight;
		desc->descBubbleLoc.ptH = gLayout.layoFieldBox.rectLeft
				- gFieldCenter.ptH + col * kBubbleLargeWidth;
		}
}


#ifdef TRANSLUCENT_HEALTH_BARS
/*
**	TransludentBarView::DoDraw()
**
**	draw the hit and spell point bars
*/
void
TransludentBarView::DoDraw()
{
	DTSRect viewBounds;
	GetBounds( &viewBounds );
	Erase( &viewBounds );
	
# ifdef USE_OPENGL
	Draw1TranslucentBar( mValue, mMax, mColor, 0, viewBounds );
}

void
TransludentBarView::Draw1TranslucentBar( int mValue, int mMax,
	const DTSColor * mColor, ushort alpha, const DTSRect& viewBounds )
{
# endif	// USE_OPENGL
	
		// OGL path assumes blending enabled and blend function set up correctly
		// DTS path assumes it is part of class TransludentBarView
	
	// light gray border
	DTSColor color( 0xEEEE, 0xEEEE, 0xEEEE );
	
# ifdef USE_OPENGL
	if ( gUsingOpenGL )
		{
		glColor4us( color.rgbRed, color.rgbGreen, color.rgbBlue, alpha );
		glBegin( GL_LINE_LOOP );
			glVertex2f( viewBounds.rectLeft + 0.5f, viewBounds.rectTop + 0.5f );
			glVertex2f( viewBounds.rectRight - 0.5f, viewBounds.rectTop + 0.5f );
			glVertex2f( viewBounds.rectRight - 0.5f, viewBounds.rectBottom - 0.5f );
			glVertex2f( viewBounds.rectLeft + 0.5f, viewBounds.rectBottom - 0.5f );
		glEnd();
		}
	else
# endif	// USE_OPENGL
		{
		SetForeColor( &color );
		Frame( &viewBounds );
		}
	
	// inset one pixel
	DTSRect	bar	= viewBounds;
	bar.Inset( 1, 1 );
	
	// total usable dimension
	int width = bar.rectRight - bar.rectLeft;
	
	if ( HB_Color_Variable == ( gPrefsData.pdHBColorStyle & 0x0FFFF ) )
		{
		// one popular style:
		// draw the full part in color according to the value experimentally
		// determined to simulate the nametag color as server sends 'em
		if ( mValue >= 228 )
			{
			// server sends white in this range, but
			// the bar looks better in green
			color.Set( 0, 0x8000, 0 );
			}
		else if ( mValue >= 150 )
			{
			// green
			color.Set( 0, 0x8000, 0 );
			}
		else if ( mValue >= 49 )
			{
			// yellow
			color.Set( 0x8000, 0x8000, 0 );
			}
		else if ( mValue >= 0 )
			{
			// red
			color.Set( 0x8000, 0, 0 );
			}
		else
			{
			// white? -- doesn't really matter anyhow
			color.Set( 0xffff, 0xffff, 0xffff );
			}
		
# ifdef USE_OPENGL
		if ( gUsingOpenGL )
			glColor4us( color.rgbRed, color.rgbGreen, color.rgbBlue, alpha );
		else
# endif	// USE_OPENGL
			SetForeColor(&color);
		}
	else
		{
		// another style: bars have individual colors
		// (the view is drawn once before being fully set, so protect against that)
		if ( mColor )
			{
# ifdef USE_OPENGL
			if ( gUsingOpenGL )
				glColor4us( mColor->rgbRed,
					mColor->rgbGreen, mColor->rgbBlue, alpha );
			else
# endif	// USE_OPENGL
				SetForeColor( mColor );
			}
		}
	
	DTSRect bounds = bar;
	bounds.rectRight = bar.rectLeft + width * mValue / 255;
# ifdef USE_OPENGL
	if ( gUsingOpenGL )
		glRecti( bounds.rectLeft, bounds.rectTop,
				 bounds.rectRight, bounds.rectBottom );
	else
# endif	// USE_OPENGL
		Paint( &bounds );
	
	// and the unavailable part
	bounds.rectLeft 	= bar.rectLeft + width * mMax / 255;
	bounds.rectRight	= bar.rectRight;
	color.Set( 0xCC00, 0xA000, 0x4000 );	// yellowish
# ifdef USE_OPENGL
	if ( gUsingOpenGL )
		{
		glColor4us( color.rgbRed, color.rgbGreen, color.rgbBlue, alpha );
		glRecti( bounds.rectLeft, bounds.rectTop,
				 bounds.rectRight, bounds.rectBottom );
		}
	else
# endif	// USE_OPENGL
		{
		SetForeColor( &color );
		Paint( &bounds );
		}
	
	// draw the divider lines
	color.Set( 0xEEEE, 0xEEEE, 0xEEEE );
	
# ifdef USE_OPENGL
	bool insideBeginEnd = false;
	if ( gUsingOpenGL )
		glColor4us( color.rgbRed, color.rgbGreen, color.rgbBlue, alpha );
	else
# endif	// USE_OPENGL
		SetForeColor( &color );
	
	DTSLine divider;
	if ( mValue != 0 && mValue != 255 )
		{
# ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
			if ( not insideBeginEnd )
				{
				glBegin( GL_LINES );
				insideBeginEnd = true;
				}
			glVertex2f( bar.rectLeft + width * mValue / 255 + 0.5f,
						bar.rectTop + 0.5f );
			glVertex2f( bar.rectLeft + width * mValue / 255 + 0.5f,
						bar.rectBottom + 0.5f );
			}
		else
# endif	// USE_OPENGL
			{
			divider.Set( bar.rectLeft + width * mValue / 255, bar.rectTop,
						 bar.rectLeft + width * mValue / 255, bar.rectBottom );
			Draw( &divider );
			}
		}
	
	if ( mMax != 0 && mMax != mValue && mMax != 255 )	// was: if ( mMax != mValue )
		{
# ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
			if ( not insideBeginEnd )
				{
				glBegin( GL_LINES );
				insideBeginEnd = true;
				}
			glVertex2f( bar.rectLeft + width * mMax / 255 + 0.5f,
						bar.rectTop + 0.5f );
			glVertex2f( bar.rectLeft + width * mMax / 255 + 0.5f,
						bar.rectBottom + 0.5f );
			}
		else
# endif	// USE_OPENGL
			{
			divider.Set( bar.rectLeft + width * mMax / 255, bar.rectTop,
					 bar.rectLeft + width * mMax / 255, bar.rectBottom );
			Draw( &divider );
			}
		}
# ifdef USE_OPENGL
	if ( gUsingOpenGL )
		{
		if ( insideBeginEnd )
			{
			glEnd();
				// this isn't needed here;
				// included in case function gets expanded.
				// optimizer should remove it.
				// But the static analyzer doesn't like it, so it's gone
//			insideBeginEnd = false;
			}
		}
# endif	// USE_OPENGL
}
#endif	// TRANSLUCENT_HEALTH_BARS


/*
**	CLOffView::DrawBar()
**
**	draw the hit and spell point bars
*/
void
CLOffView::DrawBar( const DTSRect * box, int value, int max )
{
	// draw the good in green
	int width = box->rectRight - box->rectLeft;
	DTSRect bounds = *box;
	bounds.rectRight = box->rectLeft + width * value / 255;
	DTSColor color( 0x0000, 0x8000, 0x0000 );
	SetForeColor( &color );
	Paint( &bounds );
	
	// draw the lost but recoverable in grey
	bounds.rectLeft  = bounds.rectRight;
	bounds.rectRight = box->rectLeft + width * max / 255;
	
	// note this is NOT the same as DTSColor::grey50
	// because the system's default 256-color palette
	// quantizes them very differently! Bah.
	color.Set( 0x8000, 0x8000, 0x8000 );
	SetForeColor( &color );
	Paint( &bounds );
	
	// draw the injured in yellow
	bounds.rectLeft  = bounds.rectRight;
	bounds.rectRight = box->rectRight;
	color.Set( 0x8000, 0x8000, 0x0000 );
	SetForeColor( &color );
	Paint( &bounds );
}


void
TouchHealthBars()
{
#ifdef TRANSLUCENT_HEALTH_BARS
	if ( CLWindow * win = static_cast<CLWindow *>( gGameWindow ) )
		win->winOffView.statusBarTouch = 0;
#endif
}


#ifdef TRANSLUCENT_HEALTH_BARS
void
CLOffView::DrawTranslucentHealthBars()
{
	bool bWantBars = not ( gPrefsData.pdHBColorStyle >> 16 );
	if ( not bWantBars )
		return;
	
	uint statusValue = ( (int) gFrame->mHP * gFrame->mHPMax ) +
					   ( (int) gFrame->mSP * gFrame->mSPMax ) +
					   ( (int) gFrame->mBalance * gFrame->mBalanceMax );
	
	if ( statusValue != kStatusBarFull )
		statusBarTouch = 0;
	else
	if ( not statusBarTouch )
		statusBarTouch	= GetFrameCounter() + kStatusBarShowDelay;
	
	if ( not statusBarTouch || GetFrameCounter() < statusBarTouch )
		{
		DTSRect	bar;
# ifdef USE_OPENGL
		if ( gUsingOpenGL )
			bar.Set( 0, 0,
					gLayout.layoHPBarBox.rectRight
						- gLayout.layoHPBarBox.rectLeft,
					gLayout.layoHPBarBox.rectBottom
						- gLayout.layoHPBarBox.rectTop + 2 );
		else
# endif	// USE_OPENGL
			offStatusBar.GetBounds( &bar );
		
		int barWidth  = bar.rectRight  - bar.rectLeft;
		int barHeight = bar.rectBottom - bar.rectTop;
		
		DTSColor barColors[3];
		barColors[0].Set( 0, 0x8000, 0 );	// hp: green
		barColors[1].Set( 0, 0, 0x8000 );	// balance: blue
		barColors[2].Set( 0x8000, 0, 0 );	// sp: red
		
		int h = 0, v = 0, deltah = 0, deltav = 0;
		
		const DTSRect field( gLayout.layoFieldBox );
		
		switch ( gPrefsData.pdHBPlacement )
			{
			case HB_Placement_Bottom:
				// bars spread across bottom
				
				// figure left-over space after 3 instances of the bar
				deltah = (field.rectRight - field.rectLeft) - 3 * barWidth;
				
				// now apportion that space into 6 slots,
				// one on each side of the bars
				deltah /= 6;
				
				// the first bar is 1/6th of the way across, 20 pixels up
				v = field.rectBottom - 20 - barHeight;
				h = field.rectLeft + deltah;
				
				// each bar is offset from the last by its width
				// plus 2/6 of the unused space with no vertical offset
				deltah = 2 * deltah + barWidth;
				deltav = 0;
				break;
			
			case HB_Placement_LowerLeft:
				// another style: put the bars in the lower-left
				// start 20 pixels in from the lower-left corner
				v = field.rectBottom - 20;
				v -= 3 * barHeight + 4 + 4;
				h = field.rectLeft + 20;
				
				// stacked vertically, 4 pixel interval
				deltah = 0;
				deltav = 4 + barHeight;
				break;
			
			case HB_Placement_LowerRight:
				// same, but lower right
				v = field.rectBottom - 20;
				v -= 3 * barHeight + 4 + 4;
				h = field.rectRight - 20 - barWidth;
				deltah = 0;
				deltav = 4 + barHeight;
				break;
			
			case HB_Placement_UpperRight:
				// same, but upper right
				v = field.rectTop + 20;
				h = field.rectRight - 20 - barWidth;
				deltah = 0;
				deltav = 4 + barHeight;
				break;
			}
		
		const uchar kAlphaTrigger = 76; // 76 is 30% of 255
		DTSRect offStatusBarBounds;
# ifdef USE_OPENGL
		const GLushort kAlpha75 = 0xBFFF;
		const GLushort kAlpha50 = 0x7FFF;
		if ( gUsingOpenGL )
			{
			enableBlending();
			disableAlphaTest();
			disableTexturing();
			}
		else
# endif	// USE_OPENGL
			offStatusBar.GetBounds( &offStatusBarBounds );
		
		bar.Move(h, v );
# ifdef USE_OPENGL
		if ( gUsingOpenGL )
			offStatusBarView.Draw1TranslucentBar( gFrame->mHP,
				gFrame->mHPMax, &barColors[0],
				gFrame->mHP < kAlphaTrigger ? kAlpha75 : kAlpha50, bar) ;
		else
# endif	// USE_OPENGL
			{
			offStatusBarView.DrawTransBar( gFrame->mHP,
				gFrame->mHPMax, &barColors[0] );
			SelectiveBlit( this, &offStatusBar, &offStatusBarBounds, &bar,
				gFrame->mHP < kAlphaTrigger ? kBlitter25 : kBlitter50 );
			}

		bar.Offset(deltah, deltav );
# ifdef USE_OPENGL
		if ( gUsingOpenGL )
			offStatusBarView.Draw1TranslucentBar( gFrame->mBalance,
				gFrame->mBalanceMax, &barColors[1],
				gFrame->mBalance < kAlphaTrigger ? kAlpha75 : kAlpha50, bar );
		else
# endif	// USE_OPENGL
			{
			offStatusBarView.DrawTransBar( gFrame->mBalance,
				gFrame->mBalanceMax, &barColors[1] );
			SelectiveBlit( this, &offStatusBar, &offStatusBarBounds, &bar,
				gFrame->mBalance < kAlphaTrigger ? kBlitter25 : kBlitter50 );
			}

		bar.Offset(deltah, deltav);
# ifdef USE_OPENGL
		if ( gUsingOpenGL )
			offStatusBarView.Draw1TranslucentBar( gFrame->mSP,
				gFrame->mSPMax, &barColors[2],
				gFrame->mSP < kAlphaTrigger ? kAlpha75 : kAlpha50, bar );
		else
# endif	// USE_OPENGL
			{
			offStatusBarView.DrawTransBar( gFrame->mSP,
				gFrame->mSPMax, &barColors[2] );
			SelectiveBlit( this, &offStatusBar, &offStatusBarBounds, &bar,
				gFrame->mSP < kAlphaTrigger ? kBlitter25 : kBlitter50 );
			}
	}
}
#endif	// TRANSLUCENT_HEALTH_BARS


#ifdef	TRANSLUCENT_HAND_ITEMS

/*
**	CLOffView::DrawTranslucentHandObjects()
*/
void
CLOffView::DrawTranslucentHandObjects()
{
	DTSKeyID id = gLeftPictID;
	if ( id )
		{
		if ( const ImageCache * cache = CachePicture( id ) )
			{
			DTSRect src = cache->icImage.cliImage.imageBounds;
			src.rectBottom = src.rectTop + cache->icHeight;
			
			int side = cache->icHeight;
			DTSRect	itemBox;
			itemBox.Size( side, side );
			itemBox.Move( gLayout.layoFieldBox.rectLeft + 16,
						  gLayout.layoFieldBox.rectTop + 16 );
			SelectiveBlit( this,
				&cache->icImage.cliImage, &src, &itemBox, kBlitter50 );
			}
		}
	
	id = gRightPictID;
	if ( id )
		{
		if ( const ImageCache * cache = CachePicture( id ) )
			{
			DTSRect src = cache->icImage.cliImage.imageBounds;
			src.rectBottom = src.rectTop + cache->icHeight;
			Draw( &cache->icImage.cliImage,
				kImageCopyFast, &src, &gLayout.layoRightObjectBox );
			
			int side = cache->icHeight;
			DTSRect	itemBox;
			itemBox.Size( side, side );
			itemBox.Move( gLayout.layoFieldBox.rectRight - 16 - side,
						  gLayout.layoFieldBox.rectTop + 16 );
			SelectiveBlit( this,
				&cache->icImage.cliImage, &src, &itemBox, kBlitter50 );
			}
		}
}
#endif	// TRANSLUCENT_HAND_ITEMS


/*
**	CLOffView::DrawHandObjects()
**
**	draw the objects in the hands
*/
void
CLOffView::DrawHandObjects()
{
	SetForeColor( &DTSColor::black );
	
	// left hand
	DTSKeyID id = gLeftPictID;
	if ( 0 == id )
		id = kEmptyLeftHandPict;
	DTSRect src;
	const ImageCache * cache = CachePicture( id );
	if ( cache )
		{
		cache->icImage.cliImage.GetBounds( &src );
		src.rectBottom = src.rectTop + cache->icHeight;
		Draw( &cache->icImage.cliImage, kImageCopyFast,
				&src, &gLayout.layoLeftObjectBox );
		
		if ( id == gSelectedItemPictID )
			{
			SetHiliteMode();
			Invert( &gLayout.layoLeftObjectBox );
			}
		}
	else
		Erase( &gLayout.layoLeftObjectBox );
	
	// right hand
	id = gRightPictID;
	if ( 0 == id )
		id = kEmptyRightHandPict;
	cache = CachePicture( id );
	if ( cache )
		{
		cache->icImage.cliImage.GetBounds( &src );
		src.rectBottom = src.rectTop + cache->icHeight;
		Draw( &cache->icImage.cliImage, kImageCopyFast,
				&src, &gLayout.layoRightObjectBox );
		
		if ( id == gSelectedItemPictID )
			{
			SetHiliteMode();
			Invert( &gLayout.layoRightObjectBox );
			}
		}
	else
		Erase( &gLayout.layoRightObjectBox );
	
#if 0
	// selected, if different
	id = gSelectedItemPictID;
	if ( id
	&&	 id != gLeftPictID
	&&	 id != gRightPictID )
		{
// I don't see _where_ to draw this image
// unless the layout changes
		
		DTSRect dst = ?
		
		cache = CachePicture( id );
		DTSRect src = cache->icImage.cliImage.imageBounds;
		src.rectBottom = src.rectTop + cache->icHeight;
		
		Draw( &cache->icImage.cliImage, kImageCopyFast, &src, &dst );
		SetHiliteMode();
		Invert( &dst );
		}
#endif  // 0
}


/*
 **	CLOffView::DrawMovieMode()
 **
 **	display movie mode captions, for recording or playback speed
 */
void
CLOffView::DrawMovieMode()
{
	if ( GetFrameCounter() > offMovieSignBlink )
		{
		offMovieSignShown = not offMovieSignShown;
		offMovieSignBlink = GetFrameCounter() + KMovieSignBlinkDelay;
		}
	if ( offMovieSignShown )
		{
		static const char * const speedNames[] =
			{
			"PAUSE",
			"PLAY",
			"FFWD",
			"TURBO"
			};
		const char * text = CCLMovie::IsRecording() ? "REC"
						  : speedNames[ CCLMovie::GetReadSpeed() ];
		
#ifdef USE_OPENGL
		if ( gUsingOpenGL )
			{
			disableBlending();
			disableAlphaTest();
			disableTexturing();
			glColor3us( 0xFFFF, 0, 0 );
			drawOGLText( gLayout.layoFieldBox.rectLeft + 5,
					gLayout.layoFieldBox.rectTop + 5 + kTextAscent,
					geneva9BoldListBase, text );
			}
		else
#endif  // USE_OPENGL
			{
			SetForeColor( &DTSColor::red );
			
			// use 9 point geneva bold
			SetFont( "Geneva" );
			SetFontSize( 9 );
			SetFontStyle( bold );
			Draw( text, gLayout.layoFieldBox.rectLeft + 5,
				 gLayout.layoFieldBox.rectTop + 5 + kTextAscent, kJustLeft );
			
			// restore the color/style
			SetForeColor( &DTSColor::black );
			SetFontStyle( normal );
			}
		}
}


/*
**	CLOffView::DrawMovieProgress()
**
**	display a progress gauge for the current movie
*/
void
CLOffView::DrawMovieProgress( const DTSRect& box )
{
	int cur, tot;
	CCLMovie::GetPlaybackPosition( cur, tot );
	
	float frac = float( cur ) / float( tot );
	int hPos = box.rectLeft + (int)( frac * (box.rectRight - box.rectLeft) );
	
	DTSLine	meter;
	meter.Set( hPos - 1, box.rectTop, hPos - 1, box.rectBottom - 1 );
	if ( hPos > box.rectLeft )
		{
		SetForeColor( &DTSColor::black );
		Draw( &meter );
		}
	
	SetForeColor( &DTSColor::red );
	meter.Offset( 1, 0 );
	Draw( &meter );
	
	SetForeColor( &DTSColor::black );
	meter.Offset( 1, 0 );
	if ( hPos < box.rectRight )
		Draw( &meter );
}


/*
**	CLOffView::DrawFieldInactive()
**
**	gray out the playfield box, after communications to the server have ceased
*/
void
CLOffView::DrawFieldInactive()
{
#ifdef USE_OPENGL
	if ( gUsingOpenGL )
		{
		disableBlending();
		enableAlphaTest();
		disableTexturing();
		glEnable( GL_POLYGON_STIPPLE );
		static const GLubyte gray25[] = {
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
			0x22, 0x22, 0x22, 0x22, 0x88, 0x88, 0x88, 0x88,
		};
		glPolygonStipple( gray25 );
		glColor3us( 0x0000, 0x0000, 0x0000 );
		glRecti( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectTop,
				gLayout.layoFieldBox.rectRight, gLayout.layoFieldBox.rectBottom );
		
		glDisable( GL_POLYGON_STIPPLE );
		}
	else
#endif	// USE_OPENGL
		{
		DTSPattern pat;
		pat.SetGray25();
		SetPattern( &pat );
		SetForeColor( &DTSColor::black );
		SetPenMode( kPenModeTransparent );
		Paint( &gLayout.layoFieldBox );
		// no need to restore pattern/colors/penmode, because of the upcoming Reset()
		}
}


/*
**	GetGWMouseLoc()
**
**	return the position of the mouse relative to the center of the play-field
*/
void
GetGWMouseLoc( DTSPoint * pt )
{
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	win->GetMouseLoc( pt );
	
	pt->ptV -= gFieldCenter.ptV;
	pt->ptH -= gFieldCenter.ptH;
}


/*
**	TextView::InitTV()
**
**	Initialize the text box view
*/
void
TextView::InitTV()
{
	Init( kCLDepth );
	DTSError result = tvImage.Init( nullptr,
						kTextViewWidth, kTextViewHeight, kCLDepth );
	if ( noErr == result )
		result = tvImage.AllocateBits();
	if ( noErr == result )
		{
		SetImage( &tvImage );
		Show();
		}
	SetErrorCode( result );
}


/*
**	TextView::DoDraw()
**
**	draw a text string
**	make sure the character width table is initialized
*/
void
TextView::DoDraw()
{
	// erase
	DTSRect viewBounds;
	GetBounds( &viewBounds );
	Erase( &viewBounds );
	
	// bail if there is no text to draw
	const char * text = tvText;
	if ( not text )
		return;
	
	// use 9 point geneva
	SetFont( "Geneva" );
	SetFontSize( 9 );
	
	// make sure the character widths table is initialized
	// (couldn't do this earlier since we need a valid grafport)
	if ( 0 == gCharWidths[ (uint) 'A' ] )
		{
		FontInfo fi;
		::GetFontInfo( &fi );
		
		uchar * ptr = gCharWidths;
		char buff[2];
		buff[1] = '\0';
		for ( uint ch = 0;  ch < sizeof gCharWidths;  ++ch )
			{
			buff[0] = ch;
			int width = GetTextWidth( buff );	// or ::CharWidth()?
			*ptr++ = width;
			}
		Erase( &viewBounds );
		}
	
	// draw the text
	CLStyleRecord style;
	if ( tvFriends )
		SetUpStyle( kMsgFriendBubble, &style );
	else
		SetUpStyle( kMsgBubble, &style );
	
	SetForeColor( &style.color );
	Draw( text, 0, kTextAscent, kJustLeft );
}


#if DTS_LITTLE_ENDIAN
// dunno where best to put this, so i'll put it here

static inline void
SwapEndian( DTSRect& r )
{
	r.rectTop    = SwapEndian( r.rectTop );
	r.rectLeft   = SwapEndian( r.rectLeft );
	r.rectBottom = SwapEndian( r.rectBottom );
	r.rectRight  = SwapEndian( r.rectRight );
}


// also needed by CreateDummyImages() in Update.cp
void
SwapEndian( Layout& l )
{
	l.layoWinHeight = SwapEndian( l.layoWinHeight );
	l.layoWinWidth  = SwapEndian( l.layoWinWidth );
	l.layoPictID    = SwapEndian( l.layoPictID );
	
	SwapEndian( l.layoFieldBox );
	SwapEndian( l.layoInputBox );
	SwapEndian( l.layoHPBarBox );
	SwapEndian( l.layoSPBarBox );
	SwapEndian( l.layoAPBarBox );
	SwapEndian( l.layoTextBox );
	SwapEndian( l.layoLeftObjectBox );
	SwapEndian( l.layoRightObjectBox );
}
#endif  // DTS_LITTLE_ENDIAN


/*
**	InitLayout()
**
**	initialize the layout
*/
DTSError
InitLayout()
{
#if 0  // small window option no longer supported
	DTSKeyID id = kLargeLayoutID;
	if ( not gPrefsData.pdLargeWindow )
		id = kSmallLayoutID;
#endif  // 0
	
	DTSError result = gClientImagesFile.Read( kTypeLayout, kLargeLayoutID,
											 &gLayout, sizeof gLayout );
	if ( noErr == result )
		{
#if DTS_LITTLE_ENDIAN
		SwapEndian( gLayout );
#endif
		gFieldCenter.ptV = (gLayout.layoFieldBox.rectTop
						 + gLayout.layoFieldBox.rectBottom) / 2;
		gFieldCenter.ptH = (gLayout.layoFieldBox.rectLeft
						 + gLayout.layoFieldBox.rectRight) / 2;
		}
	
	return result;
}

extern void RecordCmd( ulong cmdNum );		// in Main_cl.cp


/*
**	ExtractImportantData()
**
**	extract the important data from the drawstate message
**	do it here and now because we might not have the trashable data later
*/
void
ExtractImportantData()
{
	// extract the acknowledged command number
	DataSpool * ds = gDSSpool;
	
//	int mark = ds->GetMark();
	
	// get the old frame. will be used to handle frame deltas
//	CCLFrame * oldFrame	= &gFrames[ gFrameIndex ];
	
	// get new frame index in our table
	++gFrameIndex;
	if ( kMaxStoredFrame == gFrameIndex )
		gFrameIndex = 0;
	
	// This is the new frame to be
	gFrame = &gFrames[ gFrameIndex ];
	
	// unpack it!
	gFrame->ReadKeyFromSpool( ds );
	
	if ( not gFrame->IsValid() )
		{
		ShowMessage( "Invalid frame; bailing" );
		gDSSpool = nullptr;
		return;
		}
	
	// that's it: do usual stuff, but without the spool!
	ulong ackCmdNum = gFrame->mAckCmdNum;
	RecordCmd( ackCmdNum );
	
	// save the frame number to be sent back with the input
	int lastAckFrame = gAckFrame;
	int newAckFrame = gFrame->mAckFrame;
	gFrameCounter = newAckFrame;
	
	if ( not CCLMovie::IsReading() )
		{
		++gNumFrames;
		++gPrefsData.pdNumFrames;
		int lostframes = newAckFrame - lastAckFrame - 1;
		if ( lastAckFrame != 0
		&&   lostframes   >  0 )
			{
//			ShowMessage( "dropped frames between %d and %d",
//				(int) lastAckFrame, (int) newAckFrame );
			gLostFrames             += lostframes;
			gPrefsData.pdLostFrames += lostframes;
			}
		}
	
//	** shadow testing **
#if 0
	SetShadowAngle( -1, gFrameCounter % 360, 0 );
#endif
	
	// oh yuck, packets received out of order
	if ( newAckFrame <= lastAckFrame )
		{
		ShowMessage( "gah! out of order" );
		gDSSpool = nullptr;
		return;
		}
	gAckFrame = newAckFrame;
	
	// ignore the resend frame for now
	int resend = gFrame->mResentFrame;
	
	// clear the send string if the server got it
	ClearSendString( ackCmdNum );
	
	// update lighting
	gNightInfo.SetFlags( gFrame->mLightFlags );
	
	// extract the descriptors, pictures, and mobiles
	ExtractDescriptors( gFrame );
	ExtractFramePictures( gFrame );
	ExtractFrameMobiles(  gFrame );
	
	// extract the state data
	ExtractStateData( gFrame, lastAckFrame, resend );
}


/*
**	ExtractDescriptors()
**
**	extract the descriptors
*/
void
ExtractDescriptors( const CCLFrame * ds )
{
	for ( uint count = 0;  count < ds->mNumDesc;  ++count )
		{
		const CCLFrame::SFrameDesc& desc = ds->mDesc[ count ];
		
		// point into the table
		DescTable * target = gDescTable + desc.mIndex;
		
#ifdef AUTO_HIDENAME
		target->descSeenFrame	= 0;				// never seen this guy, prolly
		target->descNameVisible	= kName_Visible;	// Show his name
#endif
		
		// is this a new descriptor?
		if ( (uint) target->descID != desc.mPictID
		||   (uint) target->descType != desc.mType
		||   (uint) target->descNumColors != desc.mNumColors
		||   strcmp( target->descName, desc.mName )
		||   memcmp( target->descColors, desc.mColors, desc.mNumColors ) )
			{
			// setup the descriptor
			target->descID             = desc.mPictID;
			target->descCacheID        = desc.mPictID;
			target->descMobile		   = nullptr;
			target->descType		   = desc.mType;
			target->descSize           = kPlayerSize / 2;
			target->descBubbleType     = 0;
			target->descBubbleLanguage = 0;
			target->descBubbleCounter  = 0;
			target->descBubblePos      = kBubblePosNone;
			target->descBubbleText[0]  = '\0';
			StringCopySafe( target->descName, desc.mName, sizeof target->descName );
			target->descNumColors      = desc.mNumColors;
			memcpy( target->descColors, desc.mColors, desc.mNumColors );
			target->descPlayerRef	   = nullptr;
			
			// get the size of the mobile
			if ( const ImageCache * ic = CachePicture( target ) )
				target->descSize = ic->icBox.rectRight / 32;
			}
		}
}


/*
**	ExtractFramePictures()
**
**	extract the frame pictures
*/
void
ExtractFramePictures( const CCLFrame * ds )
{
	// initialize the queue variables
	gPicQueCount = 0;
	gPicQueStart = gPictureQueue;
	
	// queue every picture for drawing
	// but don't overflow gPictureQueue
	int numpicts = ds->mNumPict + ds->mNumPictAgain;
	if ( numpicts > kMaxPicQueElems )
		numpicts = kMaxPicQueElems;
	
	for ( int nnn = 0;  nnn < numpicts;  ++nnn )
		{
		const CCLFrame::SFramePict& pict = ds->mPict[ nnn ];
		++gPicQueCount;
		Queue1Picture( nnn, pict.mPictID, pict.mH, pict.mV );
		}
	
	// debugging message
	if ( gPrefsData.pdShowImageCount )
		ShowMessage( "Drawing %d/%d images.",
			(int) ds->mNumPict, (int) ds->mNumPictAgain );
	
#if CL_DO_NIGHT
	//*teo after other pictures, maybe draw night, too
	// only if we are not viewing a movie - BMH
	// clamp to prefs' max night level
	
	int nightPct = gNightInfo.GetLevel();
	int limit = gNightInfo.GetEffectiveNightLimit();
	if ( nightPct > limit )
		nightPct = limit;
	
# ifdef USE_OPENGL
#  ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
	if ( false )
#  endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
	if ( nightPct > 0 && (gUsingOpenGL || not CCLMovie::IsReading()) )
# else
	if ( nightPct > 0 && not CCLMovie::IsReading() )
# endif	// USE_OPENGL
		{
		if ( nightPct < 38 )
			Queue1Picture( gPicQueCount++, kNight25Pct,  0, 0 );	// 25
		else
		if ( nightPct < 63 )
			Queue1Picture( gPicQueCount++, kNight50Pct,  0, 0 );	// 50
		else
		if ( nightPct < 88 )
			Queue1Picture( gPicQueCount++, kNight75Pct,  0, 0 );	// 75
		else
			Queue1Picture( gPicQueCount++, kNight100Pct, 0, 0 );	// 100
		}
	
# ifdef CL_DO_WEATHER
	// experimental weather effect (by Torx)
	int weatherImg = gWeatherInfo.GetImage();
	if ( weatherImg )
		{
		// check if we are outdoors (outdoors is when we are allowed shadows)
		if ( not ( gWeatherInfo.GetOutdoorOnly()
					&& (gFrame->mLightFlags & kLightNoShadows) ) )
			Queue1Picture( gPicQueCount++, weatherImg, 0, 0 );
		}
# endif	// CL_DO_WEATHER
#endif  // CL_DO_NIGHT
}


#ifdef OGL_SEASONS
/*
**	GetSeasonalPicture()
**	preliminary experimental feature
*/
static DTSKeyID
GetSeasonalPicture( DTSKeyID orig )
{
	DTSKeyID remappedPictID = orig;
	switch ( orig )
		{
		case 863:	// these are all variants of a certain tree image
		case 861:
		case 859:
		case 215:
			switch ( OGL_SEASONS )
				{
				case kFall:	  remappedPictID = 863; break;
				case kSummer: remappedPictID = 861; break;
				case kSpring: remappedPictID = 859; break;
				case kWinter: remappedPictID = 215; break;
				}
			break;
		
		case 864:
		case 862:
		case 860:
		case 217:
		case 865:
			switch ( OGL_SEASONS )
				{
				case kFall:		remappedPictID = 864; break;
				case kSummer:	remappedPictID = 862; break;
				case kSpring:	remappedPictID = 860; break;
				case kWinter:
				//				remappedPictID = 217;
								remappedPictID = 865;
								break;
				}
			break;
		
		case 857:	// terrain tiles
		case 876:
		case 858:
		case 491:
			switch ( OGL_SEASONS )
				{
				case kFall:		remappedPictID = 857; break;
				case kSummer:	remappedPictID = 876; break;
				case kSpring:	remappedPictID = 858; break;
				case kWinter:	remappedPictID = 491; break;
				}
			break;
		
		default:
			// if it isn't in our list, don't mess with it
			break;
		}
	
	return remappedPictID;
}
#endif  // OGL_SEASONS


/*
**	Queue1Picture()
**
**	Insert one picture into the queue
*/
void
Queue1Picture( int count, DTSKeyID pictID, int horz, int vert  )
{
	// be safe - protect ourselves from pictures that can't be queued
	// assign them a plane that puts them at the end
	PictureQueue * pq = &gPictureQueue[ count ];
	pq->pqPlane = 0x7FFF;
	pq->pqCache = nullptr;
	
	// seasonal picture experiment
#ifdef OGL_SEASONS
	pictID = GetSeasonalPicture( pictID );
#endif
	
	// cache the picture in memory
	// bail if we could not use this picture for some reason.
	ImageCache * cache = CachePicture( pictID );
	if ( not cache )
		return;
	
	// get the plane for the picture
	int plane = cache->icImage.cliPictDef.pdPlane;
	
	// search backwards in the list
	for ( ;  count > 0;  --count )
		{
		// stop when we find a picture belonging to a plane before us
		int test = pq[-1].pqPlane;
		if ( test < plane )
			break;
		
		// stop when we find a picture that is in the same plane
		// and above us (i.e., north of us).
		if ( test == plane
		&&   pq[-1].pqVert <= vert )
			break;
		
		--pq;
		pq[1] = pq[0];
		}
	
	// put this picture in the queue
	pq->pqPlane = plane;
	pq->pqCache = cache;
	pq->pqHorz  = horz;
	pq->pqVert  = vert;
}


/*
**	ExtractFrameMobiles()
**
**	extract the mobiles
*/
void
ExtractFrameMobiles( const CCLFrame * ds )
{
	// debugging message
	if ( gPrefsData.pdShowImageCount )
		ShowMessage( "Drawing %d mobiles.", (int) ds->mNumMobile );
	
	// load the mobiles
	gNumMobiles = 0;
	DSMobile * dsm = &gDSMobile[0];
	DescTable * newme = nullptr;
	
	for ( uint count = 0;  count < ds->mNumMobile;  ++count, ++dsm )
		{
		const CCLFrame::SFrameMobile& mob = ds->mMobile[ count ];
		
		dsm->dsmIndex  = mob.mIndex;
		dsm->dsmState  = mob.mState;
		dsm->dsmHorz   = mob.mH;
		dsm->dsmVert   = mob.mV;
		dsm->dsmColors = mob.mColors;
		
		gDescTable[ mob.mIndex ].descMobile = dsm;
		
		++gNumMobiles;
		
		//
		// Here, I look for a descriptor that is a player and is at 0,0
		// ... in other words, the descriptor for "me"
		// it should be safe in all situations that I tested so far
		// and until I have the 'me' descriptor index in the protocol
		// it will be needed
		//
		if ( not newme )
			{
			if ( 0 == dsm->dsmHorz && 0 == dsm->dsmVert )
				{
				DescTable * desc = &gDescTable[ dsm->dsmIndex ];
				if ( kDescPlayer == desc->descType )
					{
					// found a likely "me" candidate
					newme = desc;
					gThisMobile	= dsm;
					}
				}
			}
		}
	
	//
	// If we found a valid descriptor, set it as the 'me' global
	//
	if ( newme && newme != gThisPlayer )
		{
		// Here's how to always make me a friend of myself. It'll
		// work fine for normal players who can't ghost. But if a
		// ghost stands exactly atop another player, then gThisPlayer
		// will cycle back and forth every frame.
#if 0
			static int previousFriendState = kFriendNone;
			
			// *fixme
			// what do I do with the previous 'me'?
			// Do I leave him alone? or unfriend him?
			// Or maybe remember his old friendship state and
			// restore that? (gets expensive)
			if ( gThisPlayer )
				SetPlayerIsFriend( gThisPlayer, previousFriendState );
			
			// in any case, DO befriend the new me
			previousFriendState = GetPlayerIsFriend( newme );
			SetPlayerIsFriend( newme, kFriendLabel1 );
#endif  // 0
		
		// We could remove the old descriptor name from
		// the player list if name is different; would handle the
		// renaming-GM bug.
		
		gThisPlayer = newme;
		// ShowMessage( "New me is %s", newme->descName );
		}
}


/*
**	ExtractStateData()
**
**	extract the text, sounds, and inventory data
*/
void
ExtractStateData( const CCLFrame * ds, int lastAckFrame, int receivedResend )
{
	// check if we're looking for a resend
	int resendFrame = gResendFrame;
	if ( resendFrame != 0 )
		{
		// we are looking for a resend
		if ( not receivedResend || receivedResend != resendFrame )
			return;								// this is not it!
		
		// yay! we just received the resend we were looking for
		// stop looking for it
		// and carry on as normal
		// we're fully recovered
		gResendFrame = 0;
		}
	else
	if ( lastAckFrame != 0 )
		{
		// check for a missed frame
		resendFrame = lastAckFrame + 1;
		if ( gAckFrame != resendFrame )
			{
			// we lost a frame
			// begin the recovery process
			gResendFrame = resendFrame;
			return;
			}
		}
	else
		{
		if ( not CCLMovie::IsReading() )
			{
			// this happens all the time when reading the first frame of a movie
			ShowMessage( "can this even happen? "
				"last ack %d ack %d resend %d gResend %d.",
				lastAckFrame, (int) gAckFrame, receivedResend, resendFrame );
			}
		}
	
	int spoolsize = ds->mStateLen;
	const uchar * spoolptr = ds->mStateData;
	
	switch ( gStateMode )
		{
		for(;;)
			{
			// there's a slim but non-zero chance that the two bytes that make
			// up the state-size word end up in different UDP packets, so we
			// have to collect them individually.
			
			case kStateSizeHi:		// grab hi-order byte of state size word
				if ( spoolsize <= 0 )
					{
					gStateMode = kStateSizeHi;
					return;
					}
				gStateExpectSize = *spoolptr++ << 8;
				--spoolsize;
				FALLTHRU_OK;
			
			case kStateSizeLo:		// grab lo-order byte of ditto
				if ( spoolsize <= 0 )
					{
					gStateMode = kStateSizeLo;
					return;
					}
				gStateExpectSize |= *spoolptr++;
				gStateCurSize = 0;
				--spoolsize;
				if ( gStateExpectSize > 1024
				||   gStateExpectSize < 4 )
					{
					ShowMessage( "StateData: expected size possibly bad: %d.",
						gStateExpectSize );
					}
				FALLTHRU_OK;
			
			case kStateData:
				{
				// we have the complete size word, so we know how much
				// more data to expect in this frame
				int expsize = gStateExpectSize;
				
				// maybe enlarge our state-data buffer
				if ( gStateMaxSize < expsize )
					{
					uchar * newptr = NEW_TAG("StateData") uchar[ expsize ];
					if ( not newptr )
						{
						ShowMessage( "StateData: state is now horked." );
						gStateMode = kStateHorked;
						return;
						}
					delete[] gStatePtr;
					gStatePtr     = newptr;
					gStateMaxSize = expsize;
					}
				
				// fill it up
				int cursize = gStateCurSize;
				uchar * dst = gStatePtr + cursize;
				int sizetoget = expsize - cursize;
				if ( sizetoget > spoolsize )
					sizetoget = spoolsize;
				
				memcpy( dst, spoolptr, sizetoget );
				spoolptr  += sizetoget;
				spoolsize -= sizetoget;
				cursize   += sizetoget;
				gStateCurSize = cursize;
				if ( cursize < expsize )
					{
					gStateMode = kStateData;
					return;
					}
				HandleStateData();
				}
			} // end of for(;;) loop
		
		case kStateHorked:
			ShowMessage( "StateData: state is horked." );
			break;
		}
}


/*
**	HandleStateData()
**
**	handle the state data
*/
void
HandleStateData()
{
	// info text
	const uchar * ptr = HandleInfoText( gStatePtr );
	
	// bubbles
	int count;
	for ( count = *ptr++;  count > 0;  --count )
		ptr = Handle1Bubble( ptr );
	
	// sounds
	for ( count = *ptr++;  count > 0;  --count )
		ptr = Handle1Sound( ptr );
	
	// inventory
	if ( CCLMovie::IsReading() && CCLMovie::GetVersion() <= MakeLong( 119, 0 ) )
		{
		// do nothing; ignore incompatible inventory commands in old movies
		}
	else
		ptr = HandleInventory( ptr );
}


/*
**	CheckMacroInterrupt()
**
**	allow the server to interrupt a macro
*/
static bool
CheckMacroInterrupt( const char * text, size_t len )
{
	const char wantText[] = "/m_interrupt";
	const size_t wantLen = sizeof(wantText) - 1;
	
	if ( len < wantLen )
		return false;
	
	if ( strncmp( text, wantText, wantLen ) )
		return false;
	
	// we got the magic command
	InterruptMacro();
	return true;
}


/*
**	HandleInfoText()
**
**	Handle info text
*/
uchar *
HandleInfoText( uchar * ptr )
{
	// we expect info text to come in as "\rsample text\rmore text"
	// would like to call ShowInfoText() once per line
	// but the tune thingy doesn't like the \r at the front
	char * current = reinterpret_cast<char *>( ptr );
	for(;;)
		{
		// stop at end of string
		char * start = current;
		char ch = *start;
		if ( 0 == ch )
			break;
		
		// step over the leading \r if any
		if ( '\r' == ch )
			++start;
		
		// find the terminating \r if any
		char * end = strchr( start, '\r' );
		if ( end )
			*end = '\0';
		
		// was this a special client instruction?
		size_t len = strlen( start );
		
		// this do...while is only here so I can 'break' after each test
		do
			{
#if CL_DO_NIGHT
			if ( gNightInfo.ScanServerMessage( start, len ) )
				{
				gNightInfo.SetShadows();
				break;
				}
#endif
			
#ifdef HANDLE_MUSICOMMANDS
			if ( gTuneQueue.HandleCommand( start, len ) )
				break;	// do nothing, it's handled
#endif
			
#if ENABLE_MUSIC_FILES			
			if ( CheckMusicCommand( start, len ) )
				break;	// scripted song change: ditto
#endif
			
			if ( CheckMacroInterrupt( start, len ) )		// ditto
				break;
			
			if ( CheckLudification( start, len ) )			// ditto
				break;
			
#ifdef MULTILINGUAL
			// check if the server tells us to switch language
			if ( CheckRealLanguageId( start, len ) )
				break;
#endif

#ifdef CL_DO_WEATHER
			// check if we received weather information
			if ( CheckWeather( start, len ) )
				break;
#endif // CL_DO_WEATHER
			
			// not a special instruction, so treat as regular info text
			ShowInfoText( start );
			} while ( false );
		
		// restore the terminating \r
		// update current
		if ( end )
			{
			*end = '\r';
			current = end;
			}
		else
			{
			current = start + len;
			break;
			}
		}
	
	return reinterpret_cast<uchar *>( current + 1 );
}


#if defined(MULTILINGUAL)
/*
** 	ExtractTypeThinkBubble()
**
**	Return the type of the think bubble (language independent)
**	(kBubbleThink_none (old style english only think), kBubbleThink_thinkto, etc..)
**
**	Also fixes the the Bubble-text by removing the type-marker bepp-string.
**	e.g. "Torx" + kBEPPChar + "t_tt to you: Hello"
**	  becomes
**	     "Torx to you: Hello"
**
**	See the comment of ParseThinkText() in PlayerWin_cl.cp for some more information.
**
*/
static int
ExtractTypeThinkBubble( DescTable * target )
{
	const char * text = target->descBubbleText;
	char bepptype [ sizeof kBEPPThinkBubble_think ];
	int thinktype = kBubbleThink_none;
	int pos = 0;
	
	// look for colon or BEPP - whatever is found first.
	while ( pos < strlen(text) && text[pos] != ':' && text[pos] != kBEPPChar )
		++pos;
	
	// we found the colon first or the end of the string?
	if ( text[pos] == ':' || pos == strlen(text) )
		// it's the old style then - no special handling needed, as there is no BEPP
		return kBubbleThink_none;
	
	// now pos is at the start of the BEPP. Extract it completely
	// but check first that the whole BEPP string would fit
	if ( pos + strlen(kBEPPThinkBubble_think) >= strlen(text) )
		return kBubbleThink_none;
	
	BufferToStringSafe( bepptype, sizeof bepptype,
		text + pos, sizeof kBEPPThinkBubble_think );
	
	// match the extracted BEPPstr with our predefined think types
	if ( not strncmp( bepptype,
				kBEPPThinkBubble_think, sizeof kBEPPThinkBubble_think ) )
		{
		thinktype = kBubbleThink_think;
		}
	
	if ( not strncmp( bepptype,
				kBEPPThinkBubble_thinkto, sizeof kBEPPThinkBubble_thinkto ) )
		{
		thinktype = kBubbleThink_thinkto;
		}
	
	if ( not strncmp( bepptype,
				kBEPPThinkBubble_thinkclan, sizeof kBEPPThinkBubble_thinkclan ) )
		{
		thinktype = kBubbleThink_thinkclan;
		}
	
	if ( not strncmp( bepptype,
				kBEPPThinkBubble_thinkgroup, sizeof kBEPPThinkBubble_thinkgroup ) )
		{
		thinktype = kBubbleThink_thinkgroup;
		}
	
	// we saved the type of the think, now we can remove that extra
	// piece of information from the original bubble text (by copying
	// the text backwards over the bepp string)
	StringCopySafe( target->descBubbleText + pos,
		target->descBubbleText + pos + sizeof kBEPPThinkBubble_think - 1,
		sizeof target->descBubbleText - pos - sizeof kBEPPThinkBubble_think );
	
	return thinktype;
}
#endif // MULTILINGUAL


/*
**	Handle1Bubble()
**
**	spool out a bubble
**
**	Bubble data format from server:
**		ubyte	desc-table index of bubble's source
**		ubyte	bubble type flags (packed):
**			6 bits for basic type (kBubbleNormal ... kBubbleNarrate)
**			1 bit  for language extension marker (1 = kBubbleNotCommon)
**			1 bit  for near/far (1 = kBubbleFar)
**
**	 -- now it gets a bit complicated --
**
**	 o If kBubbleNotCommon is set, the next datum is:
**		ubyte	language code (also packed):
**			6 bits for language index (kBubbleHafling ... kBubbleMystic)
**			2 bits for known/size code:
**				00 = recipient understands the bubble's language (text is valid)
**				01 = short  utterance in a language not understood (no text sent)
**				10 = medium ditto
**				11 = long   ditto
**
**	 o If kBubbleFar is set, the next data is:
**		ushort	horizontal offset of bubble source
**		ushort	vertical ditto
**
**	 o Finally there is the actual bubble text:
**		ubyte+	text of bubble, zero-terminated, using MacRoman encoding.
**				(may be zero-length; see note below).
**
**	Notes: The server sends no bubble text if you don't understand the
**	language (to prevent rogue clients from eavesdropping on conversations
**	they shouldn't). In that case, the client must synthesize an appropriate
**	replacement. On the other hand, even though you might not understand,
**	say, kLanguageSanskrit, you should still be able to distinguish between
**	a monosyllabic grunt and a major oration! Hence the bubble size code
**	flags: the client can use those to choose the proper bubble image and a
**	suitable synthetic content string -- see FillUnknownLanguage().
**	
**	Some players may have lost the ability to understand the Common
**	language, e.g. for RP reasons. So that GMs can nevertheless communicate
**	reliably, we added the Standard language, which is implicitly understood
**	by everyone, always.
**	
**	This routine needs to do two main jobs: first, obviously, to cause the
**	correct visual bubble image to appear in the playfield, and second, to
**	generate the textual transcript for the text window. The first task is
**	downright easy in comparison to the second!
*/
const uchar *
Handle1Bubble( const uchar * ptr )
{
	// get the index into the descriptor table
	int index = *ptr++;
	
	// get the type
	int type = *ptr++;
	int lang = 0;
	
	// preserve compatibility for old movies
	// (it may be time to retire some of these fixups;
	// v142 was a *long* time ago...)
	if ( CCLMovie::IsReading() )
		{
		if ( CCLMovie::GetVersion() < MakeLong(138, 0) )
			{
			// patch up pre-138 bubbles
			// what is now kBubbleHalfling (8) used to be kBubbleFarYell
			// what is now kBubbleSylvan (9) .. kBubbleMagic + 1 (16)
			//	used to be kBubbleHalfling .. kBubbleMagic
			// this brings them into v138-141 format.
			
			// (I'm writing these with numerals not constants because
			// the values of those constants have changed over time. These
			// are the values that were current before 138.)
			if ( type > 7 )
				{
				if ( 8 == type )
					type = 16 + 2;
				else
					--type;
				}
			}
		
		// patch up 138-141 bubbles to v142 format
		if ( CCLMovie::GetVersion() < MakeLong(142, 0) )
			{
/* bubble type mapping, 141->142
id		142			141
0		normal		normal
1		whisper		whisper
2		yell		yell
3		thought		thought
4		act			act
5		monster		monster
6		act2		act2
7		ponder		ponder
8		narrate		normal + halfling
9		n/a			+ sylvan
10					+ people
...					...
15					+ magic
16		n/a			far normal
...					... (same as above, + far)
31					far + magic
*/
			int newtype = 0;
			// strip out far-ness
			if ( type >= 16 && type <= 31 )
				{
				newtype = kBubbleFar;
				type -= 16;
				}
			
			// strip out languages
			if ( type >= 8 && type < 16 )
				{
//				newtype |= kBubbleNormal;	// no-op
				lang = (type - kBubblePonder) | kBubbleCodeKnown;
				}
			else
				{
				// pick up the basic shapes
				newtype |= type;
				}
			
			// now we have all we need
			type = newtype;
			}
		}
	
	// get the language if this is not common
	if ( type & kBubbleNotCommon )
		lang = *ptr++;
	else
	if ( lang != 0 )
		{
		// the movie-patching code has determined the language;
		// now adjust the type
		type |= kBubbleNotCommon;
		// and convert the language code from 1-based to 0-based
		--lang;
		}
	
	// get the location if this is a far yell
	int horz = 0;
	int vert = 0;
	if ( type & kBubbleFar )
		{
		horz = ( ptr[0] << 8 ) + ptr[1];
		vert = ( ptr[2] << 8 ) + ptr[3];
		ptr += 4;
		}
	
	// check the string for non-white space characters
	const char * text = nullptr;
	if ( ( type & kBubbleNotCommon ) != kBubbleNotCommon
	||   ( lang & kBubbleCodeMask  ) == kBubbleCodeKnown )
		{
		text = reinterpret_cast<const char *>(ptr);
		for(;;)
			{
			int ch = *ptr++;
			if ( 0 == ch )
				return ptr;
			
			if ( ch != ' '
			&&   ch != '\t'
			&&   ch != '\r'
			&&   ch != '\n' )
				{
				break;
				}
			}
		}
	
	// point into the table
	DescTable * target = gDescTable;
	
	bool bMustPlayThinkToSound = false;
	
	// change the index if this is a thought
	if ( type == kBubbleThought )
		{
		// replace the oldest thought
		DescTable * thought = target + kDescTableBaseSize;
		int minAge = INT_MAX;
		for ( int step = kDescTableBaseSize;
			  step < kDescTableSize;  ++step, ++thought )
			{
			int age = thought->descBubbleCounter;
			if ( age < minAge )
				{
				minAge = age;
				index  = step;
				}
			}
		}
	
	// stuff the bubble text info into the descriptor
	target += index;
	target->descBubbleLoc.Set( horz, vert );
	target->descBubbleType     = type;
	target->descBubbleLanguage = lang;
	target->descBubbleCounter  = kTextDelayLength;
	
#ifdef OGL_USE_TEXT_FADE
	if ( gUsingOpenGL )
		target->descBubbleCounter += kTextFadeLength;
#endif	// OGL_USE_TEXT_FADE
	
	target->descBubblePos = kBubblePosNone;
	
	if ( text )
		StringCopySafeNoDupSpaces( target->descBubbleText,
			text, sizeof target->descBubbleText );
	else
		{
#if USE_WHISPER_YELL_LANGUAGE
		FillUnknownLanguage( type, lang,
			target->descName, target->descBubbleText );
#else
		FillUnknownLanguage( lang, target->descName, target->descBubbleText );
#endif	// USE_WHISPER_YELL_LANGUAGE
		}
	
	// stuff it into the text window
	char buff[ kMaxNameLen + kMaxDescTextLength + 256 ];
	int type2;
	
#if USE_WHISPER_YELL_LANGUAGE
	if ( text || ((type & kBubbleTypeMask) == kBubbleYell) )
#else
	if ( text )
#endif	// USE_WHISPER_YELL_LANGUAGE
		{
		type2 = type & kBubbleTypeMask;
		}
	else
		type2 = kBubbleRealAction;
	
	// concoct alternatives for unnamed mobiles
	const char * speakerName = target->descName;
	if ( speakerName[ 0 ] == '\0' )
		{
		switch ( target->descType )
			{
			case kDescMonster:
				speakerName = _(TXTCL_BUBBLE_UNKNOWN_MONSTER);
				break;
			case kDescPlayer:
				speakerName = _(TXTCL_BUBBLE_UNKNOWN_PLAYER);
				break;
			default:
				speakerName = _(TXTCL_BUBBLE_UNKNOWN_SOMETHING);
				break;
			}
		}
	
	switch ( type2 )
		{
		case kBubbleNormal:
			{
			// Person says, "message"
			char punct =
				target->descBubbleText[ strlen( target->descBubbleText ) - 1];
			const char * verb;
			switch ( punct )
				{
				case '?':	verb = _(TXTCL_BUBBLE_ASK);		break;
				case '!':	verb = _(TXTCL_BUBBLE_EXCLAIM);	break;
				default:	verb = _(TXTCL_BUBBLE_SAY);		break;
				}
			
			if ( 0 == (type & kBubbleNotCommon) )
				snprintf( buff, sizeof buff,
					"%s %s, \"%s\"", speakerName, verb, target->descBubbleText );
			else
				{
				int langindex = (lang & kBubbleLanguageMask) + 1;
				if ( langindex < 0  ||  langindex >= kNumLanguages )
					langindex = 0;
				else
					langindex = lang + 1;
				
				// wacky special case
				if ( (kBubbleCommon + 1) == langindex )
					snprintf( buff, sizeof buff,
						"%s %s, \"%s\"",
						speakerName,
						verb,
						target->descBubbleText );
				else
					snprintf( buff, sizeof buff,
						// "%s %s in %s, \"%s\"",
						_(TXTCL_BUBBLE_SAY_IN_LANG),
						speakerName,
						verb,
						gLanguageTableShort[langindex],
						target->descBubbleText
						);
				}
			}
			break;
		
		case kBubbleMonster:
			// Monster growls, "message"
			snprintf( buff, sizeof buff,
				// "%s growls, \"%s\"",
				_(TXTCL_BUBBLE_GROWL),
				speakerName, target->descBubbleText );
			break;
		
		case kBubbleWhisper:
			// Person whispers, "message"
#if USE_WHISPER_YELL_LANGUAGE
			{
			if ( 0 == (type & kBubbleNotCommon) )
				snprintf( buff, sizeof buff,
					// "%s whispers, \"%s\"",
					_(TXTCL_BUBBLE_WHISPER),
					speakerName, target->descBubbleText );
			else
				{
				int langindex = (lang & kBubbleLanguageMask) + 1;
				if ( langindex < 0  ||  langindex >= kNumLanguages )
					langindex = 0;
				else
					langindex = lang + 1;
				
				// wacky special case
				if ( (kBubbleCommon + 1) == langindex )
					snprintf( buff, sizeof buff,
						// "%s whispers, \"%s\"",
						_(TXTCL_BUBBLE_WHISPER),
						speakerName,
						target->descBubbleText );
				else
					snprintf( buff, sizeof buff,
						// "%s %s in %s, \"%s\"",
						_(TXTCL_BUBBLE_WHISPER_IN_LANG),
						speakerName,
						gVerbTable[kLanguageWhisperVerb][langindex],
						gLanguageTableShort[langindex],
						target->descBubbleText
						);
				}
			}
#else
			snprintf( buff, sizeof buff, _(TXTCL_BUBBLE_WHISPER),
				speakerName, target->descBubbleText );
#endif	// USE_WHISPER_YELL_LANGUAGE
			break;
		
		case kBubbleYell:
			// Person yells, "stuff"
#if USE_WHISPER_YELL_LANGUAGE
			{
			// Person whispers, "message"
			// for yell only, this is the string displayed in the text log
			if ( 0 == (type & kBubbleNotCommon) )
				snprintf( buff, sizeof buff,
					// "%s yells, \"%s\"",
					_(TXTCL_BUBBLE_YELL),
					speakerName, target->descBubbleText );
			else
				{
				int langindex = ( lang & kBubbleLanguageMask ) + 1;
				if ( langindex < 0  ||  langindex >= kNumLanguages )
					langindex = 0;
				
				// wacky special case
				if ( (kBubbleCommon + 1) == langindex )
						/*
						this isn't right, either...
						it fixes this: 'Crius yells something, "It's 6 o'clock..."'
						But there's still a problem with people
						who don't understand Common.
						*/
					snprintf( buff, sizeof buff,
//						"%s %s, \"%s\"",
						//"%s yells, \"%s\"",
						_(TXTCL_BUBBLE_YELL),
						speakerName,
//						gVerbTable[kLanguageLogYellVerb][langindex],
						target->descBubbleText );
				else
					switch ( lang & kBubbleCodeMask )
						{
						case kBubbleUnknownShort:
							snprintf( buff, sizeof buff,
								"%s %s, \"%s\"",
								speakerName,
								gVerbTable[kLanguageLogYellVerb][langindex],
								target->descBubbleText
								);
							break;
						
						case kBubbleUnknownMedium:
						case kBubbleCodeKnown:
						default:
							snprintf( buff, sizeof buff,
								// "%s %s in %s, \"%s\"",
								_(TXTCL_BUBBLE_YELL_IN_LANG),
								speakerName,
								gVerbTable[kLanguageLogYellVerb][langindex],
								gLanguageTableShort[langindex],
								target->descBubbleText
								);
							break;
						
						case kBubbleUnknownLong:
							snprintf( buff, sizeof buff,
								// "%s %s in %s, \"%s\"",
								_(TXTCL_BUBBLE_YELL_IN_LANG),
								speakerName,
								gVerbTable[kLanguageLogYellVerb][langindex],
								gLanguageTableLong[langindex],
								target->descBubbleText
								);
							break;
						}
				}
			}
#else
			snprintf( buff, sizeof buff, _(TXTCL_BUBBLE_YELL),
				speakerName, target->descBubbleText );
#endif	// USE_WHISPER_YELL_LANGUAGE
			break;
		
		case kBubbleRealAction:
			// Person does stuff (& hope script added correct punctuation)
			snprintf( buff, sizeof buff, "%s", target->descBubbleText );
			break;
		
		case kBubblePlayerAction:
			// put parens around action to show it's not "real"
			snprintf( buff, sizeof buff, "(%s)", target->descBubbleText );
			break;
		
		case kBubblePonder:
			// Person ponders, "thoughts"
			snprintf( buff, sizeof buff, _(TXTCL_BUBBLE_PONDER),
				speakerName, target->descBubbleText );
			break;
		
		case kBubbleThought:
			{
			// Person thinks[{ to you| to your clan| to a group}], "thought"
			target->descType = kDescThoughtBubble;
			ParseThinkText( target );

#if defined( MULTILINGUAL )
			// handle also multilingual thoughts,
			// which have added BEPP-markers in the think string
			int typeThinkBubble = ExtractTypeThinkBubble( target );
#endif // MULTILINGUAL
			
			const char * ptr2 = strchr( target->descBubbleText, ':' );
			if ( not ptr2 )
				// a thought without a thinker!?!? How quaint
				StringCopySafe( buff, target->descBubbleText, sizeof buff );
			else
				{
#if defined( MULTILINGUAL )
				// Note: the following constant strings should not get
				// translated! The multilingual think system works
				// differently, and these strings are still needed for
				// ClanLord and old movie compatibilty.
#endif // MULTILINGUAL
				const char * toyou = " to you:";
				const char * toyourclan  = " to your clan:";
				const char * toyourgroup = " to a group:";
				
				// skip the colon
				++ptr2;
				if (
#if defined( MULTILINGUAL )
					kBubbleThink_thinkto == typeThinkBubble ||
#endif // MULTILINGUAL
					not strncmp( ptr2 - strlen(toyou), toyou, strlen(toyou) ) )
					{
					if ( ' ' == *ptr2 )
						++ptr2;
					
					int friends = GetPlayerIsFriend( target );
					if ( gPrefsData.pdThinkNotify
					&&   kFriendLabel1 <= friends
					&&   kFriendLabelLast >= friends )
						{
						DoNotification( target->descName, toyou, ptr2 );
						}
					
					bMustPlayThinkToSound =
						not ( kFriendBlock == friends || kFriendIgnore == friends );
					snprintf( buff, sizeof buff,
						//"%s thinks to you, \"%s\"",
						_(TXTCL_BUBBLE_TO_THINK),
						target->descName, ptr2 );
					}
				else
				if (
#if defined( MULTILINGUAL )
					kBubbleThink_thinkgroup == typeThinkBubble ||
#endif // MULTILINGUAL
					not strncmp( ptr2 - strlen(toyourgroup),
						toyourgroup, strlen(toyourgroup) ) )
					{
					if ( ' ' == *ptr2 )
						++ptr2;
					
					int friends = GetPlayerIsFriend( target );
					if ( gPrefsData.pdThinkNotify
					&&   kFriendLabel1 <= friends
					&&   kFriendLabelLast >= friends )
						{
						DoNotification( target->descName, toyourgroup, ptr2 );
						}
					
					bMustPlayThinkToSound =
						not ( kFriendBlock == friends || kFriendIgnore == friends );
					snprintf( buff, sizeof buff,
						//"%s thinks to a group, \"%s\"",
						_(TXTCL_BUBBLE_GROUP_THINK),
						target->descName, ptr2 );
					}
				else
				if (
#if defined( MULTILINGUAL )
					kBubbleThink_thinkclan == typeThinkBubble ||
#endif // MULTILINGUAL
					not strncmp( ptr2 - strlen(toyourclan),
						toyourclan, strlen(toyourclan) ) )
					{
					if ( ' ' == *ptr2 )
						++ptr2;
					
					int friends = GetPlayerIsFriend( target );
					if ( gPrefsData.pdThinkNotify
					&&   kFriendLabel1 <= friends
					&&   kFriendLabelLast >= friends )
						{
						DoNotification( target->descName, toyourclan, ptr2 );
						}
					
					bMustPlayThinkToSound =
						not ( kFriendBlock == friends || kFriendIgnore == friends );
					snprintf( buff, sizeof buff,
						// "%s thinks to your clan, \"%s\"",
						_(TXTCL_BUBBLE_CLAN_THINK),
						target->descName, ptr2 );
					}
				else
					{
					if ( ' ' == *ptr2 )
						++ptr2;
					
					snprintf( buff, sizeof buff,
						_(TXTCL_BUBBLE_THINK), target->descName, ptr2 );
					}
				}
			}
			break;
		
		case kBubbleNarrate:
			// (person): message
			snprintf( buff, sizeof buff,
				"(%s): %s", speakerName, target->descBubbleText );
			break;
		
		default:
			// unknown bubble type?!
			// Person declaims, "message"
			snprintf( buff, sizeof buff, _(TXTCL_BUBBLE_DECLAIM),
				speakerName, target->descBubbleText );
			break;
		}
	
	if ( bMustPlayThinkToSound )
		{
		const DTSKeyID kSndThinkTo = 58;
		CLPlaySound( kSndThinkTo );
		}
	
#if USE_STYLED_TEXT
	int friends;
	// I would much prefer to test for target == gThisPlayer, but that
	// will be the wrong test if it's a thought bubble. The risk of
	// doing it this way is that gThisPlayer might not be set, although
	// I don't see how that can happen.

	// Argh. turns out it CAN happen... during the first few frames, &
	// rarely at that. But when it does, it's a crasher.
	if ( gThisPlayer )
		{
		if ( target->descPlayerRef == gThisPlayer->descPlayerRef
		&&	 not gSpecialTedFriendMode )
			{
			friends = kFriendLabel1;
			}
		else
			friends = GetPlayerIsFriend( target );
		}
	else
		{
		// we don't know who WE are yet, so how can we know who our friends are?
		friends = kFriendNone;
		}
	
	// while ignored players' bubbles are hidden from screen, they still
	// appear in the text window and log. But double-ignored players'
	// bubbles are suppressed altogether. Set this bool to false to log
	// them, nevertheless.
	
	const bool NoLogIgnoredBubbles = true;
	
	if ( not ( NoLogIgnoredBubbles && (kFriendIgnore == friends) ) )
		{
		CLStyleRecord style;
		MsgClasses speechStyle = kMsgSpeech;
		if ( bMustPlayThinkToSound )
			speechStyle = kMsgThoughtMsg;
		else
		if ( gThisPlayer && (target->descPlayerRef == gThisPlayer->descPlayerRef) )
			speechStyle = kMsgMySpeech;
		else
		if ( kFriendLabel1 <= friends
		&&   kFriendLabelLast >= friends )
			{
			speechStyle = kMsgFriendSpeech;
			}
		SetUpStyle( speechStyle, &style );
		AppendTextWindow( buff, &style );
		}
#else
	AppendTextWindow( buff );
#endif  // USE_STYLED_TEXT
	
	if ( text )
		ptr += strlen( reinterpret_cast<const char *>( ptr ) ) + 1;
	
	return ptr;
}


/*
**	Handle1Sound()
**
**	spool out and play a sound
*/
const uchar *
Handle1Sound( const uchar * ptr )
{
	// after what's come before, this is almost miraculously easy
	DTSKeyID id = (ptr[0] << 8) + ptr[1];
	ptr += 2;
	CLPlaySound( id );
	
	return ptr;
}


/*
**	HandleInvCmdFull()
**	server is sending the entire inventory
*/
static const uchar *
HandleInvCmdFull( const uchar * ptr )
{
	// OK this is kind of an ugly hack
	// we save the currently-selected item just before the reset
	// then we attempt to restore later, after the list is rebuilt
	long cookie = InventorySaveSelection();
	
	ResetInventory();
	
	// get the count
	int itemcount = *ptr++;
	
	// get the equipped flags
	uint equips = 0;
	int shift = 0;
	int idx;
	for ( idx = (itemcount + 7) >> 3;  idx > 0;  --idx )
		{
		equips |= *ptr++ << shift;
		shift += 8;
		}
	
	// get the items
	for ( idx = 1;  idx <= itemcount;  ++idx )
		{
		DTSKeyID itemID = (ptr[0] << 8) + ptr[1];
		ptr += 2;
		
		bool equipped = (equips & 1);
		
		// idx must be set to 0 for non-template items
		// but must be set to the true count for template items
		// we have no idea at this point which is which
		// so send -1 to AddToInv() -- let it decide
		AddToInventory( itemID, -1, equipped );
		equips >>= 1;
		}
	InventoryRestoreSelection( cookie );
	
	return ptr;
}


/*
**	HandleInvCmdOther()
**	any of the subcommands except kInvCmdAll, to wit:
**		add, addequip, delete, equip, unequip, name
*/
static const uchar *
HandleInvCmdOther( int cmd, const uchar * ptr )
{
	// get the item id
	DTSKeyID itemid = (ptr[0] << 8) + ptr[1];
	ptr += 2;
	
	// read the index
	int itemIndex = 0;
	if ( cmd & kInvCmdIndex )
		{
		itemIndex = ptr[0];
		ptr += 1;
		cmd &= ~kInvCmdIndex;
		}
	
	switch ( cmd )
		{
		case kInvCmdAdd:
			AddToInventory( itemid, itemIndex, false );
			ptr = Handle1CustomName( itemid, itemIndex, ptr );
			break;
		
		case kInvCmdAddEquip:
			AddToInventory( itemid, itemIndex, true );
			ptr = Handle1CustomName( itemid, itemIndex, ptr );
			break;
		
		case kInvCmdDelete:
			InventoryRemoveItem( itemid, itemIndex );
			break;
		
		case kInvCmdEquip:
			InventoryEquipItem( itemid, true, itemIndex );
			break;
		
		case kInvCmdUnequip:
			InventoryEquipItem( itemid, false, itemIndex );
			break;
		
		case kInvCmdName:
			ptr = Handle1CustomName( itemid, itemIndex, ptr );
			break;
		
		default:
#ifdef DEBUG_VERSION
			ShowMessage( BULLET " unknown inv cmd (%d)! item = %d index = %d.",
				cmd, (int) itemid, itemIndex );
#endif
			break;
		}
		
	return ptr;
}


/*
**	HandleInventory()
**
**	spool in and interpret an inventory command
*/
const uchar *
HandleInventory( const uchar * ptr )
{
	// get the first command
	int cmd = *ptr++;
	
	// bail if it is none
	if ( kInvCmdNone == cmd )
		return ptr;
	
	int cmdcount = 1;
	
	// handle multiple commands
	if ( kInvCmdMultiple == cmd )
		{
		cmdcount = ptr[0];
		cmd = ptr[1];
		ptr += 2;
		}
	
	// it's unlikely but possible that there are zero multiple commands
	while ( --cmdcount >= 0 )
		{
		switch ( cmd )
			{
			case kInvCmdFull:
				ptr = HandleInvCmdFull( ptr );
				break;
			
			case kInvCmdNone:
				// do nothing, I fervently hope
#ifdef DEBUG_VERSION
				ShowMessage( BULLET " got kInvCmdNone - huh?" );
#endif
				break;
			
			case kInvCmdFull | kInvCmdIndex:
			case kInvCmdNone | kInvCmdIndex:
				// more paranoia, mostly because we don't want these cases
				// to make it through to HandleInvCmdOther()
#ifdef DEBUG_VERSION
				ShowMessage( BULLET " Inventory: got InvFull/None + index!" );
#endif
				break;
			
			default:
				ptr = HandleInvCmdOther( cmd, ptr );
				break;
			}
		
		// get the next command
		cmd = *ptr++;
		}
	
	UpdateInventoryWindow();
	
	return ptr;
}


/*
**	Handle1CustomName()
**
**	extract next string from stream
**	and set item's custom name to that
*/
const uchar *
Handle1CustomName( DTSKeyID id, int index, const uchar * ptr )
{
	const char * text = reinterpret_cast<const char *>( ptr );
	
#ifdef DEBUG_VERSION
//	ShowMessage( "Receiving custom name '%s' for %d/%d.",
//		text, (int) id, index );
#endif
	
	// set name
	InventorySetCustomName( id, index, text );
	
	// advance stream pointer to end of string
	while ( *ptr++ != '\0' )
		;
	return ptr;
}


/*
**	GetBEP()
**	extract a 3-character BEP code; convert it into a BEPPrefixID
*/
static inline BEPPrefixID
GetBEP( const char * p )
{
	const uchar * up = reinterpret_cast<const uchar *>( p );
	uint32_t bep = (up[0] << 24) | (up[1] << 16) | (up[2] << 8) | ' ';
	return static_cast<BEPPrefixID>( bep );
}


/*
**	ShowInfoText()
**
**	show text in the info area
**	and in the text window
*/
void
ShowInfoText( const char * text )
{
	// look for special don't-display prefixes (from scripts that spoof the client)
	bool dontDisplay = false;
	bool textLogOnly = false;
//	bool gmRelated   = false;
	
	// pre-parse and discard a few oddball prefixes
	BEPPrefixID pfx = GetBEP( text );
	
#if defined( MULTILINGUAL )
	// found a multilingual tag? extract translated message
	if ( kBEPPMultiLingualID == pfx )
		{
		const char * msgpnt = text + 3;
		int msgpntidx = ExtractLangText( &msgpnt, gRealLangID, nullptr );
		// note: this modifies the original message, so multi-line
		// texts will most likely not work.
		const_cast<char *>( msgpnt )[ msgpntidx ] = '\0';
		text = msgpnt;
		
		pfx = GetBEP( text );
		}
#endif // MULTILINGUAL
	
	if ( kBEPPDontDisplayID == pfx )
		{
		dontDisplay = true;
		text += 3;
		}
	else
	if ( kBEPPTextLogOnlyID == pfx )
		{
		textLogOnly = true;
		text += 3;
		}
	else
	if ( kBEPPGameMasterID == pfx )
		{
		// ignore these for now
//		gmRelated = true;
		text += 3;
		}
	
	// then look for a prefix
//	const char * prefix = nullptr;
	BEPPrefixID prefixID = static_cast<BEPPrefixID>( 0 );
	
	static const BEPPrefixID gBEPPTable[] =
		{
		kBEPPYouKilledID,
		kBEPPHelpID,
		kBEPPGameMasterID,
		kBEPPLogOnID,
		kBEPPLogOffID,
		kBEPPDepartID,
		kBEPPInfoID,
		kBEPPKarmaID,
		kBEPPKarmaReceivedID,
		kBEPPShareID,
		kBEPPUnshareID,
		kBEPPThinkID,
		kBEPPWhoID,
		kBEPPErrID,
		kBEPPHasFallenID,
		kBEPPNoLongerFallenID,
		kBEEPBardMessageID,
		kBEPPLocationID,
		kBEPPDemoID,
		kBEPPBackEndCmdID,
		kBEPPInventoryID,
		kBEPPNewsID,
		kBEPPDownloadID,
		kBEPPConfigID,
		kBEPPMonoStyleID,
#if defined( MULTILINGUAL )
		kBEPPMultiLingualID,
#endif // MULTILINGUAL
		static_cast<BEPPrefixID>( 0 )	// terminator
		};
	
	if ( kBEPPChar == *text )
		{
		// get 3 chars
		BEPPrefixID pfx = GetBEP( text );
		
		// make sure it's a legitimate BEP
		for ( int i = 0; gBEPPTable[i]; ++i )
			{
			if ( pfx == gBEPPTable[i] )
				{
				prefixID = pfx;
				text += 3;
				break;
				}
			}
		}
	
	// special stuff so old movies still work:
	else
	if ( CCLMovie::IsReading() )
		{
		const char kOldBeppChar = '/';
		if ( kOldBeppChar == *text )
			{
			int pfx = static_cast<int>( GetBEP(text) );
			// make it a v92 BEP instead
			pfx &= 0x00FFFFFF;
			pfx |= (uint(kBEPPChar) << 24);
			for ( int i = 0; gBEPPTable[i]; ++i )
				{
				if ( pfx == gBEPPTable[i] )
					{
					prefixID = gBEPPTable[i];
					text += 3;
					break;
					}
				}
			}
		}
	
	// now we have a prefix which ParseInfoText could look at.
	
	MsgClasses messageClass = kMsgDefault;
	
	// I had to move this here, from PlayersWin.cp, since these messages
	// have no BEPPrefix
	// A5 is bullet
	if ( '[' == *text || 0xA5 == * (const uchar *) text || '*' == *text )
		messageClass = kMsgHost;
	
	// confirmation messages from \thinkto
	if ( kBEPPThinkID == prefixID )
		messageClass = kMsgMySpeech;
	
	// monostyled text for special purposes
	if ( kBEPPMonoStyleID == prefixID )
		messageClass = kMsgMonoStyle;
	
	//
	// Don't show anything if the request was generated by the Players List
	// (if it doesn't have a prefix, it can't be official, so the
	// players list doesn't need to see it.)
	//
	// but do get some special messages, to be handled as default anyway
	//
	if ( ( prefixID || messageClass != kMsgDefault )
	&&	 ParseInfoText( text, prefixID, &messageClass ) )
		{
		return;
		}
	
	// if it had a don't-display prefix, then we're done
	if ( dontDisplay )
		return;
	
	// remove the player name and monster name delimiters
	// now that ParseInfoText() has seen 'em
	const char * src = text;
	
	// this is the only really worrisome cast in the entire file
	char * dst = const_cast<char *>( text );
	
	for(;;)
		{
		BEPPrefixID pf = GetBEP( src );
		if ( pf == kBEPPPlayerNameID
		||	 pf == kBEPPMonsterNameID
		||	 pf == kBEPPLocationID
		||	 pf == kBEPPClanNameID
		||	 ( CCLMovie::IsReading() && (pf == '/pn ' || pf == '/mn ')) )
			{
			src += 3;
			}
		else
			{
			if ( dst != src )
				*dst = *src;
			if ( 0 == *src )
				break;
			++src;
			++dst;
			}
		}
	
#if USE_SPEECH
	// say this msg aloud, if possible and desired
	if ( gSpeechAvailable
	&&	 gPrefsData.pdSpeaking[ messageClass ] )
		{
		// BMH - ignore leading non-alphanumeric chars from 'announce' messages.
		if ( kMsgHost == messageClass )
			Speak( text + 1 );
		else
			Speak( text );
		}
#endif  // USE_SPEECH
	
	//
	// put text into Text Log Window
	//
	
#if USE_STYLED_TEXT
	// figure out what style it should be
	CLStyleRecord infoStyle, logStyle;
	SetUpStyle( messageClass, &infoStyle );
	
	// slap-dash patch for Apr. 1 '02 monostyle stuff:
	// use 'announce' -color- (only) for monostyled 'announcements'.
	// ** remember to take this out for v228, and do it right! **
	if ( kMsgMonoStyle == messageClass
	&&	 text
	&&	 ( '*' == *text
	||     '[' == *text
	||     0xA5 == * (const uchar *) text ) )	// bullet
		{
		CLStyleRecord temp;
		SetUpStyle( kMsgHost, &temp );
		infoStyle.color = temp.color;
		}
	
	// text window needs "mono-height" lines of text, so force the text size to 12
	logStyle = infoStyle;
	logStyle.size = 12;
	
	// append the new text to the text window
	AppendTextWindow( text, &logStyle );
#else
	// append the new text to the text window
	AppendTextWindow( text );
#endif  // USE_STYLED_TEXT
	
	if ( textLogOnly )
		return;
	
	//
	// put the text in the side bar of Game window
	//
	
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	if ( not win )
		return;
	
	win->winTextField.Hide();	// avoid flickering
	
	// separate old text from newer
	win->winTextField.Insert( "\r" );
	
	// and insert the new, stylishly or not
#if USE_STYLED_TEXT
	win->winTextField.StyleInsert( text, &infoStyle );
#else
	win->winTextField.Insert( text );
#endif
	
	// trim away lines that have scrolled off the top
	if ( TFFormat * format = win->winTextField.GetFormat() )
		{
		int numLines = format->tfNumLines;
		int maxLines = gNumLines;
		if ( numLines > maxLines )
			{
			win->winTextField.SelectText( 0,
					format->tfLineStarts[ numLines - maxLines ] );
			win->winTextField.Clear();
			win->winTextField.SelectText( 0x7FFF, 0x7FFF );
			}
		delete[] (char *) format;
		}
	win->winTextField.Show();
}


/*
**	LocateMobileByPoint()
**
**	Find the mobile under the mouse.
*/
DescTable *
LocateMobileByPoint( const DTSPoint * where, int inKind )
{
	// attempt to search in reverse Z order (to the extent that
	// makes sense for mobiles)
	const DSMobile * mobile = gDSMobile + gNumMobiles - 1;
	
	for ( int nnn = gNumMobiles;  nnn > 0;  --nnn, --mobile )
		{
		DescTable * desc = gDescTable + mobile->dsmIndex;
		
		if ( inKind != kDescUnknown && desc->descType != inKind )
			continue;
		
		// check if point is in descBox or nametag
		DTSRect box = desc->descLastDstBox;
		box.rectBottom += 14;
		
		if ( where->InRect( &box ) )
			return desc;
		}
	return nullptr;
}


/*
**	SetBackColors()
**
**	Set the back colors based on the "Bright Colors" menu.
*/
void
SetBackColors()
{
    if ( gPrefsData.pdBrightColors )
        {
        gBackColorTable[kColorCodeBackWhite ].Set( 0xEEEE, 0xEEEE, 0xEEEE );
        gBackColorTable[kColorCodeBackGreen ].Set( 0x6666, 0xFFFF, 0x6666 );
        gBackColorTable[kColorCodeBackYellow].Set( 0xFFFF, 0xFFFF, 0x6666 );
        gBackColorTable[kColorCodeBackRed   ].Set( 0xFFFF, 0x6666, 0x6666 );
        gBackColorTable[kColorCodeBackBlack ].Set( 0x0000, 0x0000, 0x0000 );
        gBackColorTable[kColorCodeBackBlue  ].Set( 0x9999, 0xFFFF, 0xFFFF );
        gBackColorTable[kColorCodeBackGrey  ].Set( 0x6666, 0x6666, 0x6666 );
        gBackColorTable[kColorCodeBackCyan  ].Set( 0x0000, 0x8888, 0x8888 );
        gBackColorTable[kColorCodeBackOrange].Set( 0x9999, 0x6666, 0x0000 );
        }
    else
        {
        gBackColorTable[kColorCodeBackWhite ].Set( 0xDDDD, 0xDDDD, 0xDDDD );
        gBackColorTable[kColorCodeBackGreen ].Set( 0x8888, 0xFFFF, 0x8888 );
        gBackColorTable[kColorCodeBackYellow].Set( 0xFFFF, 0xFFFF, 0x8888 );
        gBackColorTable[kColorCodeBackRed   ].Set( 0xFFFF, 0x8888, 0x8888 );
        gBackColorTable[kColorCodeBackBlack ].Set( 0x0000, 0x0000, 0x0000 );
        gBackColorTable[kColorCodeBackBlue  ].Set( 0xCCCC, 0xFFFF, 0xFFFF );
        gBackColorTable[kColorCodeBackGrey  ].Set( 0x6666, 0x6666, 0x6666 );
        gBackColorTable[kColorCodeBackCyan  ].Set( 0x0000, 0x8888, 0x8888 );
        gBackColorTable[kColorCodeBackOrange].Set( 0x9999, 0x6666, 0x0000 );
        }

/*
	if ( gPrefsData.pdBrightColors )
		{
		gBackColorTable[kColorCodeBackWhite ].Set( 0xFFFF, 0xFFFF, 0xFFFF );
		gBackColorTable[kColorCodeBackGreen ].Set( 0x6000, 0xFFFF, 0x6000 );
		gBackColorTable[kColorCodeBackYellow].Set( 0xFFFF, 0xFFFF, 0x6000 );
		gBackColorTable[kColorCodeBackRed   ].Set( 0xFFFF, 0x6000, 0x6000 );
		gBackColorTable[kColorCodeBackBlack ].Set( 0x0000, 0x0000, 0x0000 );
		gBackColorTable[kColorCodeBackBlue  ].Set( 0x9000, 0xFFFF, 0xFFFF );
		gBackColorTable[kColorCodeBackGrey  ].Set( 0x8000, 0x8000, 0x8000 );
		gBackColorTable[kColorCodeBackCyan  ].Set( 0x0000, 0x8000, 0x8000 );
		gBackColorTable[kColorCodeBackOrange].Set( 0x9999, 0x6666, 0x0000 );
		}
	else
		{
		gBackColorTable[kColorCodeBackWhite ].Set( 0xFFFF, 0xFFFF, 0xFFFF );
		gBackColorTable[kColorCodeBackGreen ].Set( 0x8000, 0xFFFF, 0x8000 );
		gBackColorTable[kColorCodeBackYellow].Set( 0xFFFF, 0xFFFF, 0x8000 );
		gBackColorTable[kColorCodeBackRed   ].Set( 0xFFFF, 0x8000, 0x8000 );
		gBackColorTable[kColorCodeBackBlack ].Set( 0x0000, 0x0000, 0x0000 );
		gBackColorTable[kColorCodeBackBlue  ].Set( 0xC000, 0xFFFF, 0xFFFF );
		gBackColorTable[kColorCodeBackGrey  ].Set( 0x8000, 0x8000, 0x8000 );
		gBackColorTable[kColorCodeBackCyan  ].Set( 0x0000, 0x8000, 0x8000 );
		gBackColorTable[kColorCodeBackOrange].Set( 0x9999, 0x6666, 0x0000 );
		}
*/
}


/*
**	CheckLudification()
**	handle scripted command for... oh, go look it up yourself.
*/
bool
CheckLudification( const char * inCmd, size_t inLen )
{
	bool result = false;
	if ( inLen >= 4
	&&	 inCmd
	&&	 0 == strncmp( inCmd, "/af ", 4 ) )
		{
		const char * p = inCmd + 4;
		if ( 0 == strncmp( p, "on", 2 ) )
			gFlipAllBubbles = true;
		else
		if ( 0 == strncmp( p, "off", 3 ) )
			gFlipAllBubbles = false;
		result = true;
		}
	return result;
}


#ifdef MULTILINGUAL
/*
**	CheckRealLangugeId()
**	handle backend command to switch the language-id
*/
bool
CheckRealLanguageId( const char * inCmd, size_t inLen )
{
	bool result = false;
	if ( inLen >= 4 + 1		// ensure there's at least one char of actual data
	&&	 inCmd
	&&	 0 == strncmp( inCmd, "/lg ", 4 ) )
		{
		const char * p = inCmd + 4;
		gRealLangID = atoi( p );
		
		// now trigger an inventory refresh
		UpdateInventoryWindow();
#if DEBUG_VERSION
		ShowMessage( "switching language to id: %d", gRealLangID );
#endif
		result = true;
		}
	return result;
}
#endif	// MULTILINGUAL


#ifdef CL_DO_WEATHER
/*
**	CheckWeather()
**	handle backend command to control the weather
**	It looks like "/wt <imageid of effect> <outdooronly>"
**	if <outdooronly> is not 0, then image overlay will only be displayed
**	when the player is outdoors.
*/
bool
CheckWeather( const char * inCmd, size_t inLen )
{
	int image, outdooronly;
	
	// ensure there's at least one char of actual data (after '/wt ')
	if ( inLen >= 4 + 1
	&&	 inCmd
	&&	 0 == strncmp( inCmd, "/wt ", 4 ) )
		{
		const char * p = inCmd + 4;
		int found = sscanf( p, "%d %d", &image, &outdooronly );
		if ( 2 == found )
			{
			gWeatherInfo.SetWeather( image, outdooronly != 0 );
#ifdef DEBUG_VERSION
//			ShowMessage( "setting weather to img: %d, outdoorsonly: %d",
//				image, outdooronly );
#endif // DEBUG_VERSION
			return true;
			}
		}
	
	return false;
}
#endif	// CL_DO_WEATHER


/*
**	StringCopySafeNoDupSpaces()
**
**	copy the string.
**	skip leading, trailing, and duplicate spaces.
**	don't overflow the buffer.
**	terminate dst.
*/
void
StringCopySafeNoDupSpaces( char * orgdst, const char * inSrc, size_t sz )
{
	bool bLastWasSpace = true;
	char * dst = orgdst;
	char * limit = dst + sz - 1;
	const uchar * src = reinterpret_cast<const uchar *>( inSrc );
	
	for(;;)
		{
		int ch = *src;
		if ( 0 == ch )
			break;
		
		bool bWriteMe = true;
		if ( ch == '\t'
		||   ch == '\r'
		||   ch == '\n'
		||	 ch == 0x03		// enter key
		||   ch == 0xCA )	// option-space
			{
			ch = ' ';
			}
		
		if ( ' ' == ch )
			{
			if ( bLastWasSpace )
				bWriteMe = false;
			else
				bLastWasSpace = true;
			}
		else
			bLastWasSpace = false;
		
		if ( bWriteMe )
			{
			*dst++ = ch;
			if ( dst >= limit )
				break;
			}
		++src;
		}
	if ( dst != orgdst  &&   bLastWasSpace )
		--dst;
	
	*dst = '\0';
}


/*
**	SelectiveBlit()
**
**	draw an image using your favorite translucent blitter
*/
void
SelectiveBlit( DTSOffView *		view,
			   const DTSImage *	srcimage,
			   const DTSRect *	srcrect,
			   const DTSRect *	dstrect,
			   int				whichBlitter )
{
	switch ( whichBlitter )
		{
		case kBlitter25:
			QualityBlitBlend25Transparent( view, srcimage, srcrect, dstrect );
			break;
		
		case kBlitter50:
			QualityBlitBlend50Transparent( view, srcimage, srcrect, dstrect );
			break;
		
		case kBlitter75:
			QualityBlitBlend75Transparent( view, srcimage, srcrect, dstrect );
			break;
		
		case kBlitterTransparent:
		default:
			BlitTransparent( view, srcimage, srcrect, dstrect );
			break;
		}
}


/*
**	KillNotification()
**
**	remove the notification if it's active
*/
void
KillNotification()
{
	if ( gNotificationActive )
		{
		/* OSStatus err = */ ::NMRemove( &gNotif );
		bzero( &gNotif, sizeof gNotif );
		gNotificationActive = false;
		}
}


/*
**	KillNotificationInternal()
**
**	response function for the notification -- simply kills it
**	after you acknowledge the alert.
*/
static void
KillNotificationInternal( NMRec * /* nm */ )
{
	KillNotification();
}


/*
**	DoNotification()
**
**	provide a MacOS Notification upon receipt of a think-to,
**	group think, or clan think
*/
void
DoNotification( const char * senderName, const char * format, const char * thought )
{
	// If it's a friend, and the application is not in front,
	// and we receive a thinkto, show up a notification.
	
	// don't bother if we're in the foreground
	if ( not gInBack )
		return;
	
	int state = gThisMobile->dsmState;
	
	enum NotifMsgType
		{
		kNotifMsgNone,
		kNotifMsgRequest,
		kNotifMsgFull
		};
	
	// assume full notification
	NotifMsgType notifKind = kNotifMsgFull;
	
	// but only give simple message if you are sitting or fallen
	if ( kPoseSit  == state
	||	 kPoseDead == state )
		{
		notifKind = kNotifMsgRequest;
		}
	else
	// don't notify at all if lying down
	if ( kPoseLie == state )
		{
		notifKind = kNotifMsgNone;
		}
	
	if ( kNotifMsgNone == notifKind )
		return;
	
	OSStatus err;
	static char msg[ 256 ];
	
	// remove any previous one, ignore error. Prevents a crash.
	if ( gNotificationActive )
		err = ::NMRemove( &gNotif );
	
	// prepare the notification-remover UPP
#if TARGET_RT_MAC_CFM
	static NMUPP upp;
	if ( not upp )
		upp = NewNMUPP( KillNotificationInternal );
#else
	NMUPP upp = NewNMUPP( &KillNotificationInternal );
#endif
	
	gNotif.qType 	= nmType;
	gNotif.nmMark	= 1;
	gNotif.nmIcon	= nullptr;
	gNotif.nmSound	= nullptr;
	gNotif.nmStr	= reinterpret_cast<uchar *>( msg );
	gNotif.nmResp	= upp;
	gNotif.nmRefCon	= 0;
	
	if ( kNotifMsgRequest == notifKind )
		msg[0] = snprintf( msg + 1, sizeof msg - 1,
			_(TXTCL_SENDER_REQUESTS_ATTENTION), senderName );
	else
		{
		// the full text otherwise
		msg[0] = snprintf( msg + 1, sizeof msg - 1, "%s%s \"%s\"",
						senderName,
						format,
						thought );
		}
	
	// under OS X (thru 10.1.3, anyway), this causes the message to be
	// written to the console log, for no good reason that I can see.
	// It's not at all clear how to prevent that, either.
	
	err = ::NMInstall( &gNotif );
	if ( noErr == err )
		gNotificationActive = true;
}


/*
**	FillUnknownLanguage()
**
**	fill the buffer with the text corresponding to the unknown language.
*/
void
#if USE_WHISPER_YELL_LANGUAGE
FillUnknownLanguage( int type, int lang, const char * name, char * dest )
#else
FillUnknownLanguage( int lang, const char * name, char * dest )
#endif	// USE_WHISPER_YELL_LANGUAGE
{
	int lll = ( lang & kBubbleLanguageMask ) + 1;
	if ( lll < 0  ||  lll >= kNumLanguages )
		lll = 0;
	
#if USE_WHISPER_YELL_LANGUAGE
		// kBubbleNormal, kBubbleWhisper, kBubbleYell = 0, 1, 2
		// kLanguageNormalVerb, kLanguageWhisperVerb,
		//	kLanguageUnknownYellVerbFirst = 0, 1, 2
		// but i probably shouldn't assume that, hence this switch
	int ttt;
	
	switch ( type & kBubbleTypeMask )
		{
		case kBubbleNormal:
		default:
			ttt = kLanguageNormalVerb;
			break;
		
		case kBubbleWhisper:
			ttt = kLanguageWhisperVerb;
			break;
		
		case kBubbleYell:
			{
			static int unknownYellRandomizer = kLanguageUnknownYellVerbFirst;
			ttt = unknownYellRandomizer++;
			if ( unknownYellRandomizer > kLanguageUnknownYellVerbLast )
				unknownYellRandomizer = kLanguageUnknownYellVerbFirst;
			}
			break;
		}
	
	const char * verbs = gVerbTable[ ttt ][ lll ];
	
	if ( (type & kBubbleTypeMask) == kBubbleYell )
		{
		// for yell only, this is the string that gets displayed in the gamefield
		snprintf( dest, kMaxDescTextLength, "%s!", verbs );
		}
	else

#else
		{
		const char * verbs = gVerbTable[ lll ];
		}
	
#endif	// USE_WHISPER_YELL_LANGUAGE
	
	switch ( lang & kBubbleCodeMask )
		{
		// case kBubbleUnknownShort:
		default:
			snprintf( dest, kMaxDescTextLength, "%s %s.", name, verbs );
			break;
		
		case kBubbleUnknownMedium:
			snprintf( dest, kMaxDescTextLength,
				_(TXTCL_BUBBLE_UNKNOWN_MEDIUM_IN_LANG),
				name, verbs, gLanguageTableShort[lll] );
			break;
		
		case kBubbleUnknownLong:
			snprintf( dest, kMaxDescTextLength,
				_(TXTCL_BUBBLE_UNKNOWN_LONG_IN_LANG),
				name, verbs, gLanguageTableLong[lll] );
			break;
		}
}


#ifdef USE_OPENGL

void
initOpenGLGameWindowFonts( AGLContext ctx )
{
	// i HATE doing stuff like this, but I want to minimize
	// the changes i make to base classes
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	win->winOffView.CreateOGLFontDisplayLists( ctx );
}


void
deleteOpenGLGameWindowObjects()
{
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	win->winOffView.DeleteOGLObjects();
}


void
deleteNonOpenGLGameWindowObjects()
{
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	win->winOffView.DeleteNonOGLObjects();
}


/*
**	CLOffView::Make1FontList()
**	create GL display lists for drawing text in a single font style
*/
GLuint
CLOffView::Make1FontList( AGLContext ctx, FMFontFamily fontID, int fsize, Style face )
{
	GLuint base = glGenLists( kFontArraySize );
	__Check( base );
	if ( base )
		__Verify( aglUseFont( ctx, fontID, face, fsize, 0, kFontArraySize, base ) );
	
	return base;
}


/*
**	CLOffView::CreateOGLFontDisplayLists()
**	create the GL font lists for each typeface
*/
void
CLOffView::CreateOGLFontDisplayLists( AGLContext ctx )
{
	FMFontFamily fontID = FMGetFontFamilyFromName( "\pGeneva" );
	if ( kInvalidFontFamily == fontID )
		fontID = applFont;		// fallback
	
	enum { fsize = 9 };			// point size
	
	geneva9NormalListBase =
		Make1FontList( ctx, fontID, fsize, normal );
	geneva9BoldListBase =
		Make1FontList( ctx, fontID, fsize, bold );
	geneva9ItalicListBase =
		Make1FontList( ctx, fontID, fsize, italic );
	geneva9BoldItalicListBase =
		Make1FontList( ctx, fontID, fsize, bold | italic );
	geneva9UnderlineListBase =
		Make1FontList( ctx, fontID, fsize, underline );
	geneva9BoldUnderlineListBase =
		Make1FontList( ctx, fontID, fsize, bold | underline );
	geneva9ItalicUnderlineListBase =
		Make1FontList( ctx, fontID, fsize, italic | underline );
	geneva9BoldItalicUnderlineListBase =
		Make1FontList( ctx, fontID, fsize, bold | italic | underline );
}


/*
**	CLOffView::deleteOGLObjects()
**	dispose of dynamically-allocated GL objects: displaylists, textures, etc.
*/
void
CLOffView::DeleteOGLObjects()
{
	glDeleteLists( geneva9NormalListBase, kFontArraySize );
	geneva9NormalListBase = 0;
	glDeleteLists( geneva9BoldListBase, kFontArraySize );
	geneva9BoldListBase = 0;
	glDeleteLists( geneva9ItalicListBase, kFontArraySize );
	geneva9ItalicListBase = 0;
	glDeleteLists( geneva9BoldItalicListBase, kFontArraySize );
	geneva9BoldItalicListBase = 0;
	glDeleteLists( geneva9UnderlineListBase, kFontArraySize );
	geneva9UnderlineListBase = 0;
	glDeleteLists( geneva9BoldUnderlineListBase, kFontArraySize );
	geneva9BoldUnderlineListBase = 0;
	glDeleteLists( geneva9ItalicUnderlineListBase, kFontArraySize );
	geneva9ItalicUnderlineListBase = 0;
	glDeleteLists( geneva9BoldItalicUnderlineListBase, kFontArraySize );
	geneva9BoldItalicUnderlineListBase = 0;
	
	if ( nightList )
		{
		glDeleteLists( nightList, 1 );
		nightList = 0;
		nightListLevel = 0;
		nightListRedshift = -1.0f;
		}
	if ( gOGLSelectionList )
		{
		glDeleteLists( gOGLSelectionList, 1 );
		gOGLSelectionList = 0;
		}
	bindTexture2D( 0 );
	bindTexture( 0 );
	glDeleteTextures( 1, &gOGLNightTexture );	// note bindTexture just above
	gOGLNightTexture = 0;				// zero is not a valid object name
	gOGLNightTexturePriority = -1.0f;	// -1 is not a valid priority
	glDeleteLists( gLightList, 1 );
	gLightList = 0;
}


/*
**	CLOffView::DeleteNonOGLObjects()
**	clean up the night GWorlds
*/
void
CLOffView::DeleteNonOGLObjects()
{
	if ( gLightmapGWorld )
		{
		::DisposeGWorld( gLightmapGWorld );
		gLightmapGWorld = nullptr;
		}
	if ( gAllBlackGWorld )
		{
		::DisposeGWorld( gAllBlackGWorld );
		gAllBlackGWorld = nullptr;
		}
	
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
//#warning add delete circle arrays (2) here
	// delay until the code has matured a bit
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
}


#if 0
// this only works if the screen is set to 256 first
// this was used to generate the resources.  retained for future reference
void
initOGLIndexToColorTables( ushort * i2r, ushort * i2g, ushort * i2b, GLint mapsize )
{
	DTSOffView * view = &(((CLWindow *)gGameWindow)->winOffView);
	
	int savepixel = view->Get1Pixel( 0, 0 );
	for ( int i = 0;  i < mapsize;  ++i )
		{
		view->Set1Pixel( 0, 0, i );
		DTSColor rgbcolor;
		view->Get1Pixel( 0, 0, &rgbcolor );
		i2r[i] = rgbcolor.rgbRed;
		i2g[i] = rgbcolor.rgbGreen;
		i2b[i] = rgbcolor.rgbBlue;
		}
	view->Set1Pixel( 0, 0, savepixel );
	
	ShowMessage( "red" );
	for ( int i = 0;  i < mapsize;  i += 8 )
		{
		ShowMessage("\t$\"%4.4x %4.4x %4.4x %4.4x %4.4x %4.4x %4.4x %4.4x\"",
			i2r[i], i2r[i+1], i2r[i+2], i2r[i+3],
			i2r[i+4], i2r[i+5], i2r[i+6], i2r[i+7] );
		}
	
	ShowMessage( "green" );
	for ( int i = 0;  i < mapsize;  i += 8 )
		{
		ShowMessage("\t$\"%4.4x %4.4x %4.4x %4.4x %4.4x %4.4x %4.4x %4.4x\"",
			i2g[i], i2g[i+1], i2g[i+2], i2g[i+3],
			i2g[i+4], i2g[i+5], i2g[i+6], i2g[i+7]);
		}
	
	ShowMessage( "blue" );
	for ( int i = 0;  i < mapsize;  i += 8 )
		{
		ShowMessage("\t$\"%4.4x %4.4x %4.4x %4.4x %4.4x %4.4x %4.4x %4.4x\"",
			i2b[i], i2b[i+1], i2b[i+2], i2b[i+3],
			i2b[i+4], i2b[i+5], i2b[i+6], i2b[i+7]);
		}
}
#endif	// 0


void
countLightDarkCasters()
{
	gNumLightCasters = 0;
	gNumDarkCasters = 0;

	PictureQueue * pq = gPicQueStart;
	int count = gPicQueCount;
	for ( ; count > 0 ;  --count )
		{
		ImageCache * cache = pq->pqCache;
		if ( cache && cache->icImage.cliPictDef.pdFlags & kPictDefFlagEmitsLight )
			{
			if ( cache->icImage.cliPictDef.pdFlags & kPictDefFlagLightDarkcaster )
				++gNumDarkCasters;
			else
				++gNumLightCasters;
			}
		++pq;
		}
	
	const DSMobile * mobile = gDSMobile;
	int numMobiles = gNumMobiles;
			
	for ( ; numMobiles > 0 ;  --numMobiles )
		{
		if ( DescTable * desc = gDescTable + mobile->dsmIndex )
			{
			ImageCache * cache = CachePicture( desc );
			if ( cache && cache->icImage.cliPictDef.pdFlags & kPictDefFlagEmitsLight )
				{
				int state = mobile->dsmState;
				if ( not (cache->icImage.cliPictDef.pdFlags
							& kPictDefFlagOnlyAttackPosesLit)
				||   ( state < kPoseDead && state % 4 == 3 ) )
							// kPictDefFlagOnlyAttackPosesLit && attack pose
					{
					if ( cache->icImage.cliPictDef.pdFlags
							& kPictDefFlagLightDarkcaster )
						++gNumDarkCasters;
					else
						++gNumLightCasters;
					}
				}
			}
		++mobile;
		}
	
#if defined(DEBUG_VERSION) && ( defined(OGL_SHOW_IMAGE_ID) || 0 )
	ShowMessage( "found %d lightcasters, %d darkcasters",
		gNumLightCasters, gNumDarkCasters );
#endif	// DEBUG_VERSION && OGL_SHOW_IMAGE_ID
}


void
fillNightTextureObject()
{
	gUseLightMap = false;	// defensive, or merely redundant?
	
	if ( ( gUsingOpenGL && not gOGLNightTexture )
	||   ( not gUsingOpenGL && not gLightmapGWorld ) )
		{
		createLightmap();
		if ( ( gUsingOpenGL && not gOGLNightTexture )
		||   ( not gUsingOpenGL && not gLightmapGWorld ) )
			{
			return;
			}
		}
	gUseLightMap = true;
	
	if ( gNightInfo.GetEffectiveNightLimit() != 0 )
		countLightDarkCasters();
	else
		{
		gNumLightCasters = 0;
		gNumDarkCasters = 0;
		gUseLightMap = false;
		return;
		}
	
	int nightPct = gNightInfo.GetLevel();
	int limit = gNightInfo.GetEffectiveNightLimit();
	if ( nightPct > limit )
		nightPct = limit;
	
	if ( not ( 0 == nightPct && not gNumDarkCasters ) )
		{
		gUseLightMap = true;
		
		GLfloat clearColorf = (100 - nightPct) / 100.0f;
//		ushort clearColorus = clearColorf * 0xFFFF;
		GLfloat redshiftClearColorf = clearColorf
									* gNightInfo.GetMorningEveningRedshift();
		if ( redshiftClearColorf > 1.0f )
			redshiftClearColorf = 1.0f;
//		ushort redshiftus = redshiftClearColorf * 0xFFFF;
		
		bool bufferIsInverted = false;	// default settings
		
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
		CGrafPtr oldCGrafPtr;
		GDHandle oldGDHandle;
		Ptr lightmapBaseAddr;
		int lightmapRowBytes;
		
		if ( gUsingOpenGL )
			{
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
			glClearColor( redshiftClearColorf, clearColorf, clearColorf, 0.0f );
			glClear( GL_COLOR_BUFFER_BIT );
			
			if ( not gLightList )	// this should only get called once
				{
				gLightList = glGenLists( 1 );
				if ( gLightList )
					{
					const int slices = 24;	// how many slices to cut the circle into
					GLfloat rim[slices][2];
					for ( int i = 0; i < slices; ++i )
						{
						rim[i][0] = cos( i * 360.0f / slices * F_PI / 180.0f );
						rim[i][1] = sin( i * 360.0f / slices * F_PI / 180.0f );
						}
					glNewList( gLightList, GL_COMPILE );
						glBegin( GL_TRIANGLE_FAN );
							glVertex2f( 0.0f, 0.0f );
							
							glColor3f( 0.0, 0.0, 0.0 );
							for ( int i = slices; i > 0; --i )
								glVertex2fv(rim[i - 1]);
							
							glVertex2fv( rim[slices - 1] );
						glEnd();
					glEndList();
					}
				else
					{
					gUseLightMap = false;
					return;
					}
				}
			enableBlending();
			disableAlphaTest();
			disableTexturing();
			
			glBlendFunc( GL_ONE, GL_ONE );	// we want the light combining everywhere
							// well, not anymore, but...
							// see below about blend subtract
			if ( gOGLUseDither )
				glEnable( GL_DITHER );
			
//			bufferIsInverted = false;	// default settings
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
			}
		else
			{
// hmmm.
// not using one of the DTS constructs, so I can't use SetForeColor,
// Paint, SetBackColor should I use native OS calls, or try to find the
// closest dts class and use that?
			::GetGWorld( &oldCGrafPtr, &oldGDHandle );
			::SetGWorld( gLightmapGWorld, nullptr );
			PixMapHandle pmh = ::GetGWorldPixMap ( gLightmapGWorld );
			::LockPixels( pmh );
				// the gworld was not created with the pixPurge flag,
				// so it can't be purged, so LockPixels can't fail (right?)
			lightmapBaseAddr = ::GetPixBaseAddr( pmh );
			lightmapRowBytes = ::GetPixRowBytes( pmh ) & 0x3FFF;
				// carbon + 8.5; will this be a problem?
			
			if ( clearColorf >= 0.5f )
				{
				RGBColor background = { redshiftus, clearColorus, clearColorus };
				::RGBBackColor( &background );
				::EraseRect( DTS2Mac(&gLayout.layoFieldBox) );
				::RGBBackColor( DTS2Mac(&DTSColor::white) );
				}
			}
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS

		// here's what's left of the old night
		//
		// for night from 0.0-0.5, it's not used at all.  this helps
		// avoid saturation, and lets the more subtle special effects
		// like healer pulses have more of a chance of being seen. the
		// (current) nominal exile pseudo-torch is 0.5 intensity, so the
		// light is at 1.0 (saturated) at the exile.
		//
		// for night 0.5-1.0, we'll put in a scaled down (night - 0.5)
		// version of the old effect, so that the ambient light right at
		// the exile is remains 0.5 regardless of nightlevel. combined
		// with the pseudo-torch, each exile will see themselves fully
		// lit, and other exiles at not less than 0.5 (not counting the
		// effect of darkcasters)
		//
		// if the overall night level is determined by ambient
		// moonlight, then i'm tempted to call this secondary effect
		// starlight, since like real starlight it's largly overwhelmed
		// by a bright moon, but is the primary light when there is
		// little or no moon.
		//
		// which all sounds plausible, except that it isn't affected by
		// being inside a cave or similar location where starlight
		// wouldn't reach.  oh well.
		if ( clearColorf < 0.5f )
			{
			GLfloat starlightColorf = 0.5f - clearColorf;
			
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
			if ( gUsingOpenGL )
				{
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
				
				glPushMatrix();
					glTranslatef( gFieldCenter.ptH, gFieldCenter.ptV, 0.0f );
#ifndef OGL_UNSCALED_LIGHTMAP
					glScalef( gOGLNightTexHScale, gOGLNightTexVScale, 1.0f );
#endif	//OGL_UNSCALED_LIGHTMAP
					
					glScalef( 325.0f, 325.0f, 1.0f );
					
					glColor3f( starlightColorf, starlightColorf, starlightColorf );
					glCallList( gLightList );
				glPopMatrix();
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
				}
			else
				{
				ushort starlightColorus = 0xFFFF * starlightColorf;
				RGBColor starlightColorusv
					 = { starlightColorus, starlightColorus, starlightColorus };
				Rect box = { gFieldCenter.ptV - 325, gFieldCenter.ptH - 325,
							 gFieldCenter.ptV + 325, gFieldCenter.ptH + 325 };
				RGBColor moonlightColorusv = { redshiftus, clearColorus, clearColorus };
				qdClearLightmap( moonlightColorusv, starlightColorusv );
				}
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
			}
			
		// before darkcasters, this was (1.0f - clearColorf) / 255.0f
		// which scaled each light according to the remaining "range"
		// between full daylight (1.0) and the ambient light level
		// (nightlevel).  this gave a smoother light effect. this
		// doesn't make sense with darkcasters, which would need to be
		// scaled according to the difference between ambient and full
		// dark, and neither lightcasters nor darkcasters will want to
		// be scaled differently than the "other" guys.  so we will now
		// be unscaled (full strength), which give light and dark full
		// leverage against each other, at the cost of more saturation.
		// this means the light around an exile or monster is more
		// likely to reach the limits of [0..1] and get clamped, and
		// giving an optical illusion of a ring around them at the point
		// where the light/dark stops saturating ("mach line")
		
		const GLfloat colorScalef = 1.0f / 255.0f;
			// colors are stored as unsigned bytes; we need floats
		
		// the extremes I'll go to to avoid writing a sort....
		
		const PictureQueue * pq = gPicQueStart;
		
		int nextPlane = 0x8fff;	// ilPlane is type int16_t
		
		for ( int count = gPicQueCount;  count > 0;  --count, ++pq )
			{
			ImageCache * cache = pq->pqCache;
			if ( cache
			&&   cache->icImage.cliPictDef.pdFlags & kPictDefFlagEmitsLight )
				{
				if ( cache->icImage.cliLightInfo.ilPlane < nextPlane )
					nextPlane = cache->icImage.cliLightInfo.ilPlane;
				}
			}
		
		const DSMobile * mobile = gDSMobile;
		
		for ( int numMobiles = gNumMobiles;  numMobiles > 0 ; --numMobiles, ++mobile )
			{
			DescTable * desc = gDescTable + mobile->dsmIndex;
			if ( desc )
				{
				ImageCache * cache = CachePicture( desc );
				if ( cache
				&&   cache->icImage.cliPictDef.pdFlags & kPictDefFlagEmitsLight )
					{
					int state = mobile->dsmState;
					if ( not ( cache->icImage.cliPictDef.pdFlags
								& kPictDefFlagOnlyAttackPosesLit )
							||
						( (state < 32) && ((state % 4) == 3) )
							// kPictDefFlagOnlyAttackPosesLit && attack pose
					)
						{
						if ( cache->icImage.cliLightInfo.ilPlane < nextPlane )
							nextPlane = cache->icImage.cliLightInfo.ilPlane;
						}
					}
				}
			}
		
		int plane;
		
		do {
 			plane = nextPlane;
			for ( int phase = 0; phase < 2; ++phase )	// 0 = dark, 1 = light
				{
				const PictureQueue * pq = gPicQueStart;
				
				for ( int count = gPicQueCount;  count > 0;  --count )
					{
					ImageCache * cache = pq->pqCache;
					if ( cache
					&&   cache->icImage.cliPictDef.pdFlags & kPictDefFlagEmitsLight )
						{
						if ( cache->icImage.cliLightInfo.ilPlane == plane )
							{
							bool lightdarkcaster =
									not ( cache->icImage.cliPictDef.pdFlags
											& kPictDefFlagLightDarkcaster );
							
							if ( lightdarkcaster == phase )
								{
								
#if defined(USE_OPENGL) && defined(DEBUG_VERSION) && defined(OGL_SHOW_IMAGE_ID)
	if ( lightdarkcaster )
		ShowMessage( "image casting light: %d", cache->icImage.cliPictDefID );
	else
		ShowMessage( "image casting dark: %d", cache->icImage.cliPictDefID );
#endif	// USE_OPENGL && DEBUG_VERSION && OGL_SHOW_IMAGE_ID

								int radius = cache->icImage.cliLightInfo.ilRadius;
								if ( not radius)
									radius = cache->icHeight + cache->icBox.rectRight;
									// default is H + V
								if ( not lightdarkcaster )
									radius *= 4;
									// light is stronger than dark, but dark covers more area
								
							// by promoting these here (from qdCircularLightmapElementBlitter())
							// the software GL render can benefit too
								if ( radius > gLightmapRadiusLimit )
									radius = gLightmapRadiusLimit;
								if ( radius < ( cache->icHeight + cache->icBox.rectRight ) / 2 )
									radius = ( cache->icHeight + cache->icBox.rectRight ) / 2;
								
								int jitterH = 0;
								int jitterV = 0;
								if ( cache->icImage.cliPictDef.pdFlags
										& kPictDefFlagLightFlicker )
									{
										// only doing position at present
									jitterH += GetRandom(3) - 1;	// -1,0,+1
									jitterV += GetRandom(3) - 1;	// -1,0,+1
									}
								
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
								if ( gUsingOpenGL )
									{
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
									GLfloat color[3] =
										{
										cache->icImage.cliLightInfo.ilColor[0] * colorScalef,
										cache->icImage.cliLightInfo.ilColor[1] * colorScalef,
										cache->icImage.cliLightInfo.ilColor[2] * colorScalef
										};
									
	// if blend subtract is available, we can "subtract" the darkness
	// from the lightmap, and best of all, it's completely free (same
	// speed as lightcasting), aside from disturbing the blend equation
	// state (which is bad for Apple's GL because it generates code on-the-fly)
	
	// without blend subtract, we cannot "subtract" the darkness
	// so we're going to do something kinda weird:  we're going
	// to flip the sense of everything, so dark becomes light,
	// and light becomes dark, then we'll draw the darkcaster
	// as light, then flip it all back again.
	//
	// mathematically, I want D-S (destination==lightmap minus
	// source==darkcaster) so first I use GL_ONE_MINUS_DST_COLOR * an
	// all white source rectangle, so the lightmap changes from D to
	// (1-D).  then I draw the darkcasters as light; more "darkness" is
	// drawn as more white.  thus the lightmap contains (1-D+S).
	// finally, i flip it again, so I get 1-(1-D+S),  which is just
	// (D-S)
	//
	// yes, it sounds expensive to flip the entire framebuffer TWICE,
	// but it isn't too bad in practice, at the window size we use: 
	// drawtimes on a 800MHz G4 with Radeon 7500 went from 6ms to 10ms.
	// note also that we only have to do this on the Rage128, Radeon,
	// and Radeon 7500. everything else (including, amazingly, the
	// software renderer) supports blend subtract
									
									if ( gOGLHaveBlendSubtract )
										{
										if ( lightdarkcaster )
											setBlendEquation( GL_FUNC_ADD );
												// default; source + dest
										else
											setBlendEquation( GL_FUNC_REVERSE_SUBTRACT );
												// dest - source
										}
									else
										{
										if ( lightdarkcaster == bufferIsInverted )
											{
								// these need to be the opposite; dark(false) needs inverted(true)
											// flip the whole buffer
											const GLfloat white[3] = { 1.0f, 1.0f, 1.0f };
											glColor3fv( white );
											
											glBlendFunc( GL_ONE_MINUS_DST_COLOR, GL_ZERO );
												// white color * (1-dest)
											// note that we don't need to reset the blend equation
											// because the two codepaths are mutually exclusive
											glRecti( gLayout.layoFieldBox.rectLeft,
												gLayout.layoFieldBox.rectTop,
												gLayout.layoFieldBox.rectRight,
												gLayout.layoFieldBox.rectBottom );
											glBlendFunc( GL_ONE, GL_ONE );	// back to what we had
											
											bufferIsInverted = not lightdarkcaster;
											}
										}
									
									glPushMatrix();
										glTranslatef( gFieldCenter.ptH, gFieldCenter.ptV, 0.0f );
#ifndef OGL_UNSCALED_LIGHTMAP
										glScalef( gOGLNightTexHScale, gOGLNightTexVScale, 1.0f );
#endif	//OGL_UNSCALED_LIGHTMAP
										
										glTranslatef( pq->pqHorz + jitterH,
													  pq->pqVert + jitterV, 0.0f );
										glScalef( radius, radius, 1.0f );
										glColor3fv( color );
										glCallList( gLightList );
									glPopMatrix();
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
									}
								else
									{
									RGBColor colorus = {	// convert from byte to short
										cache->icImage.cliLightInfo.ilColor[0] << 8,
										cache->icImage.cliLightInfo.ilColor[1] << 8,
										cache->icImage.cliLightInfo.ilColor[2] << 8
										};
									Point center = { gFieldCenter.ptV + pq->pqVert + jitterV,
													 gFieldCenter.ptH + pq->pqHorz + jitterH };
									
									qdLightmapCircularElementBlitter( lightmapBaseAddr,
										lightmapRowBytes,
										center, radius,
										( cache->icHeight + cache->icBox.rectRight ) / 2,
										colorus, lightdarkcaster );
									}
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
								}	// lightdarkcaster == phase
							}
						else	// plane
							{
								// if plane mismatch, check to see if it's the next plane
								// we will use for the next pass
							if ( cache->icImage.cliLightInfo.ilPlane > plane )
								{
								// if it's a plane we haven't done yet....
								if ( nextPlane == plane )
									{
									// ... and we haven't set a next plane yet, set it to this one
									nextPlane = cache->icImage.cliLightInfo.ilPlane;
									}
								else
								// if we have set a next plane already
								if ( cache->icImage.cliLightInfo.ilPlane < nextPlane )
									// ... and if this one is closer, pick it instead
									nextPlane = cache->icImage.cliLightInfo.ilPlane;
								}
							}
						}
					++pq;
					}
				
				const DSMobile * mobile = gDSMobile;
				
				for ( int numMobiles = gNumMobiles;  numMobiles > 0 ; --numMobiles )
					{
					DescTable * desc = gDescTable + mobile->dsmIndex;
					if ( desc )
						{
						ImageCache * cache = CachePicture( desc );
						if ( cache && cache->icImage.cliPictDef.pdFlags & kPictDefFlagEmitsLight )
							{
							int state = mobile->dsmState;
							if ( not ( cache->icImage.cliPictDef.pdFlags
										 & kPictDefFlagOnlyAttackPosesLit )
									||
								( (state < kPoseDead) && (state % 4 == 3) )
									// kPictDefFlagOnlyAttackPosesLit && attack pose
							)
								{
								if ( cache->icImage.cliLightInfo.ilPlane == plane )
									{
									bool lightdarkcaster = not ( cache->icImage.cliPictDef.pdFlags
											 & kPictDefFlagLightDarkcaster );
									
									if ( lightdarkcaster == phase )
										{
#if defined(USE_OPENGL) && defined(DEBUG_VERSION) && defined(OGL_SHOW_IMAGE_ID)
	if ( lightdarkcaster )
		ShowMessage( "mobile casting light:   %d", desc->descID );
	else
		ShowMessage( "mobile casting dark:   %d", desc->descID );
#endif	// USE_OPENGL && DEBUG_VERSION && OGL_SHOW_IMAGE_ID
										
										int radius = cache->icImage.cliLightInfo.ilRadius;
										if ( not radius )
				 							radius = cache->icBox.rectRight / 8;
				 								// / 16 is the actual width; we want twice that
										if ( not lightdarkcaster )
												// dark
											radius *= 4;		// darkcasters cover more area
										
										if ( kPoseDead == state )
											{
											// only true if not kPictDefFlagOnlyAttackPosesLit
											// if dead/dying, use half size & half intensity
											radius /= 2;
											}
										
							// by promoting these here (from qdCircularLightmapElementBlitter())
							// the software GL render can benefit too
										if ( radius > gLightmapRadiusLimit )
											radius = gLightmapRadiusLimit;
										if ( radius < cache->icBox.rectRight / 16 )
											radius = cache->icBox.rectRight / 16;
										
										int jitterH = 0;
										int jitterV = 0;
										if ( cache->icImage.cliPictDef.pdFlags
												& kPictDefFlagLightFlicker )
											{
												// only doing position at present
											jitterH += GetRandom(3) - 1;	// -1,0,+1
											jitterV += GetRandom(3) - 1;	// -1,0,+1
											}

#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
										if ( gUsingOpenGL )
											{
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
											GLfloat color[3] =
												{
											cache->icImage.cliLightInfo.ilColor[0] * colorScalef,
											cache->icImage.cliLightInfo.ilColor[1] * colorScalef,
											cache->icImage.cliLightInfo.ilColor[2] * colorScalef
												};
											if ( kPoseDead == state )
												{
												// only true if not kPictDefFlagOnlyAttackPosesLit
												// if dead/dying, use half size & half intensity
												color[0] *= 0.5f;
												color[1] *= 0.5f;
												color[2] *= 0.5f;
												}
											
											if ( gOGLHaveBlendSubtract )
												{
												if ( lightdarkcaster )
													setBlendEquation( GL_FUNC_ADD );
														// default; source + dest
												else
													setBlendEquation( GL_FUNC_REVERSE_SUBTRACT );
														// dest - source
												}
											else
												{
												if ( lightdarkcaster == bufferIsInverted )
													{
													// these need to be the opposite
													// flip the whole buffer
													const GLfloat white[3] = { 1.0f, 1.0f, 1.0f };
													glColor3fv( white );
													
													glBlendFunc( GL_ONE_MINUS_DST_COLOR,
														GL_ZERO );
														// white color * (1-dest)
											// note that we don't need to reset the blend equation
											// because the two codepaths are mutually exclusive
													glRecti( gLayout.layoFieldBox.rectLeft,
														gLayout.layoFieldBox.rectTop,
														gLayout.layoFieldBox.rectRight,
														gLayout.layoFieldBox.rectBottom );
													glBlendFunc( GL_ONE, GL_ONE );	// revert
													
													bufferIsInverted = not lightdarkcaster;
													}
												}
											
											glPushMatrix();
												glTranslatef( gFieldCenter.ptH,
															  gFieldCenter.ptV, 0.0f );
#ifndef OGL_UNSCALED_LIGHTMAP
												glScalef( gOGLNightTexHScale,
													gOGLNightTexVScale, 1.0f );
#endif	//OGL_UNSCALED_LIGHTMAP
												glTranslatef( mobile->dsmHorz + jitterH,
															  mobile->dsmVert + jitterV, 0.0f );
												glScalef( radius, radius, 1.0f );
												glColor3fv( color );
												glCallList( gLightList );
												
											glPopMatrix();
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
											}
										else
											{
											RGBColor colorus = {	// convert from byte to short
												cache->icImage.cliLightInfo.ilColor[0] << 8,
												cache->icImage.cliLightInfo.ilColor[1] << 8,
												cache->icImage.cliLightInfo.ilColor[2] << 8
												};
											if ( kPoseDead == state )
												{
												// only true if not kPictDefFlagOnlyAttackPosesLit
												// if dead/dying, use half size & half intensity
												colorus.red >>= 1;
												colorus.green >>= 1;
												colorus.blue >>= 1;
												}
											Point center = {
												gFieldCenter.ptV + mobile->dsmVert + jitterV,
												gFieldCenter.ptH + mobile->dsmHorz + jitterH
												};
											
											qdLightmapCircularElementBlitter(
												lightmapBaseAddr, lightmapRowBytes,
												center, radius,
												cache->icBox.rectRight / 16, colorus,
												lightdarkcaster );
											}
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
										}	// lightdarkcaster == phase
									}
								else
									{
		// if plane mismatch, check to see if it's the next plane we will use for the next pass
									if ( cache->icImage.cliLightInfo.ilPlane > plane )
										{
											// if it's a plane we haven't done yet....
										if ( nextPlane == plane )
								// ... and we haven't set a next plane yet, set it to this one
											nextPlane = cache->icImage.cliLightInfo.ilPlane;
										else
											// if we have set a next plane already
										if ( cache->icImage.cliLightInfo.ilPlane < nextPlane )
											// ... and if this one is closer, pick it instead
											nextPlane = cache->icImage.cliLightInfo.ilPlane;
										}
									}
								}
							}
						}
					++mobile;
					}
				}	// lightPhase
			} while ( nextPlane > plane );
		
			// need to honor the night setting
		if ( gNightInfo.GetEffectiveNightLimit() < 100 )
			{
			float nightCLampColorf = ( 100 - gNightInfo.GetEffectiveNightLimit() ) / 100.0f;
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
			if ( gUsingOpenGL )
				{
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
					// two passes:
		// first, subtract pdMaxNightPct from the lightmap
		// anything in the lightmap that is "lower" than pdMaxNightPct will be clamped to zero
				if ( gOGLHaveBlendSubtract )
					setBlendEquation( GL_FUNC_REVERSE_SUBTRACT );	// dest - source
				else
					{
					if ( not bufferIsInverted )
						{
						// flip the whole buffer
						const GLfloat white[3] = { 1.0f, 1.0f, 1.0f };
						glColor3fv( white );
						
						glBlendFunc( GL_ONE_MINUS_DST_COLOR, GL_ZERO );
								// white color * (1-dest)
								// note that we don't need to reset the blend equation
								// because the two codepaths are mutually exclusive
						glRecti( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectTop,
							gLayout.layoFieldBox.rectRight, gLayout.layoFieldBox.rectBottom );
						glBlendFunc( GL_ONE, GL_ONE );	// back to what we had
						
						bufferIsInverted = true;
						}
					}
				
				GLfloat nightClampColorv[3] =
					{ nightCLampColorf, nightCLampColorf, nightCLampColorf };
				glColor3fv( nightClampColorv );
				
				glRecti( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectTop,
					gLayout.layoFieldBox.rectRight, gLayout.layoFieldBox.rectBottom );
				
					// then, add pdMaxNightPct to the lightmap.
					// anything that had been clamped to 0 is now (uniformly) at pdMaxNightPct
				if ( gOGLHaveBlendSubtract )
					setBlendEquation( GL_FUNC_ADD );	// default; source + dest
				else
					{
					if ( bufferIsInverted )
						{
							// we already know this is true.
							// included to make the logic clearer, and
							// in case we need to modify this code in some way that needs this
						
						// flip the whole buffer
						const GLfloat white[3] = { 1.0f, 1.0f, 1.0f };
						glColor3fv( white );
						
						glBlendFunc( GL_ONE_MINUS_DST_COLOR, GL_ZERO );
							// white color * (1-dest)
							// note that we don't need to reset the blend equation
							// because the two codepaths are mutually exclusive
						glRecti( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectTop,
							gLayout.layoFieldBox.rectRight, gLayout.layoFieldBox.rectBottom );
						glBlendFunc( GL_ONE, GL_ONE );	// back to what we had
						glColor3fv( nightClampColorv );
						
						bufferIsInverted = false;
						}
					}
				glRecti( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectTop,
					gLayout.layoFieldBox.rectRight, gLayout.layoFieldBox.rectBottom );
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
				}
			else
				{
				qdClampLightmap( nightCLampColorf );
				}
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
			}
		
			// reset/clean up, save/restore
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
		if ( gUsingOpenGL )
			{
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
			if ( gOGLHaveBlendSubtract )
				setBlendEquation( GL_FUNC_ADD );	// default; source + dest
			
			if ( bufferIsInverted )
				{
				// flip the whole buffer
				const GLfloat white[3] = { 1.0f, 1.0f, 1.0f };
				glColor3fv( white );
				
				glBlendFunc( GL_ONE_MINUS_DST_COLOR, GL_ZERO );			// white color * (1-dest)
				glRecti( gLayout.layoFieldBox.rectLeft, gLayout.layoFieldBox.rectTop,
					gLayout.layoFieldBox.rectRight, gLayout.layoFieldBox.rectBottom );
				bufferIsInverted = false;
				}
			
			glBlendFunc( GL_SRC_ALPHA, GL_ONE_MINUS_SRC_ALPHA );	// reset
			if ( gOGLUseDither )
				glDisable( GL_DITHER );
			
				// copy the framebuffer into the texture...
			bindTexture2D( gOGLNightTexture );
			glCopyTexSubImage2D(
				GL_TEXTURE_2D, 0,
				0, 0,
#ifdef OGL_UNSCALED_LIGHTMAP
				0, 0,
				gLayout.layoFieldBox.rectRight - gLayout.layoFieldBox.rectLeft,
				gLayout.layoFieldBox.rectBottom - gLayout.layoFieldBox.rectTop
#else
				gFieldCenter.ptH - (gOGLNightTexSize / 2) - gLayout.layoFieldBox.rectLeft,
				-(gFieldCenter.ptV + (gOGLNightTexSize / 2) - gLayout.layoFieldBox.rectBottom),
				gOGLNightTexSize, gOGLNightTexSize
#endif	// OGL_UNSCALED_LIGHTMAP
				);
			
				// and tell GL that it's important enough to keep in vram, if possible
			if ( gOGLNightTexturePriority != 1.0f )
				{
				gOGLNightTexturePriority = 1.0f;
				glTexParameterf( GL_TEXTURE_2D, GL_TEXTURE_PRIORITY, gOGLNightTexturePriority );
				}
#ifdef OGL_QUICKDRAW_LIGHT_EFFECTS
			}
		else
			{
			::UnlockPixels( ::GetGWorldPixMap ( gLightmapGWorld ) );
			
			::SetGWorld( oldCGrafPtr, oldGDHandle );
			}
#endif	// OGL_QUICKDRAW_LIGHT_EFFECTS
		}
	else
		{
		gUseLightMap = false;
		
		// since we don't have a lightmap for this frame, tell GL it's ok to page it out of vram
		if ( gUsingOpenGL && (gOGLNightTexturePriority != 0.0f) )
			{
			gOGLNightTexturePriority = 0.0f;
			glPrioritizeTextures( 1, &gOGLNightTexture, &gOGLNightTexturePriority );
			}
		}
}


void
qdLightmapCircularElementBlitter(
	Ptr baseAddr,
	int rowBytes,
	const Point& center,
	int radius,
	int minimumRadius,
	const RGBColor& colorus,
	bool lightdarkcaster )
{
	// keep here, or make external, so it can be deleted in deleteNonOGLObjects() ?
	static bool firstTime = true;
	static uchar circularScale[kLightmapScaleCircleArraySize][kLightmapScaleCircleArraySize];
	if ( firstTime )
		{
		for ( int x = 0; x < kLightmapScaleCircleArraySize; ++x )
			{
			float xPixelCenter = x + 0.5f;
			for ( int y = 0; y < kLightmapScaleCircleArraySize; ++y )
				{
				float yPixelCenter = y + 0.5f;
				float dist = hypotf( xPixelCenter, yPixelCenter );
				if ( dist > kLightmapMaximumRadius )
					circularScale[y][x] = 0;
				else
					circularScale[y][x] = 0xFF *
						( kLightmapMaximumRadius - dist ) / kLightmapMaximumRadius;
				}
			}
		firstTime = false;
		}
	
	static int index[ kLightmapScaleCircleArraySize ];
	
	// precalculate the indexes into the circle array
	// if the radius is too big for the array, clamp it
	// (this is now redundant with the mainline code)
	
	if ( radius > gLightmapRadiusLimit )
		radius = gLightmapRadiusLimit;
	if ( radius < minimumRadius )
		radius = minimumRadius;
	
	if ( gCachedCircularLightmapElementRadius != radius )
		{
		// gCachedCircularLightmapElementRadius didn't gain me any speed,
		// so it may disappear in the future
		for ( int i = 0; i < radius; ++i )
			index[i] = kLightmapMaximumRadius * i / radius;
		
		for ( int i = radius; i < kLightmapScaleCircleArraySize; ++i )
			index[i] = kLightmapMaximumRadius;
		
		gCachedCircularLightmapElementRadius = radius;
		}
	
	const int bytesPerPixel = 4;
	
	int startH = center.h - radius < gLayout.layoFieldBox.rectLeft
					? gLayout.layoFieldBox.rectLeft : center.h - radius;
	int endH = center.h + radius > gLayout.layoFieldBox.rectRight
					? gLayout.layoFieldBox.rectRight : center.h + radius;
	int startV = center.v - radius < gLayout.layoFieldBox.rectTop
					? gLayout.layoFieldBox.rectTop : center.v - radius;
	int endV = center.v + radius > gLayout.layoFieldBox.rectBottom
					? gLayout.layoFieldBox.rectBottom : center.v + radius;
	
	int centerH = center.h;
	int centerV = center.v;
	
	for ( int v = startV; v < endV; ++v )
		{
		int dv = centerV > v ? centerV - v : v - centerV;
		int dvCoord = index[dv];
		
		char * p = baseAddr + v * rowBytes + ( startH - 1 ) * bytesPerPixel;
		
		for ( int h = startH; h < endH; ++h )
			{
			int dh = centerH > h ? centerH - h : h - centerH;
			int dhCoord = index[dh];
			
			p += bytesPerPixel;
			
			int colorScale = circularScale[dvCoord][dhCoord];
			
			if ( colorScale )
				{
				int scaledShiftedMaskedRed =
					( colorus.red * colorScale ) & 0x00ff0000;
				int scaledShiftedMaskedGreen =
					( ( colorus.green * colorScale ) >> 8 ) & 0x0000ff00;
				int scaledShiftedMaskedBlue =
					( ( colorus.blue * colorScale ) >> 16 ) & 0x000000ff;
				
				int oldColor = 0x00ffffff & * reinterpret_cast<int32_t *>( p );
				int red, green, blue, newColor;
				
				if ( lightdarkcaster )
					{
					// if  not already fully saturated (white)
					if ( oldColor != 0x00ffffff )
						{
						red = ( oldColor & 0x00ff0000 ) + scaledShiftedMaskedRed;
						if ( red > 0x00ff0000 )
							red = 0x00ff0000;
						
						green = ( oldColor & 0x0000ff00 ) + scaledShiftedMaskedGreen;
						if ( green > 0x0000ff00 )
							green = 0x0000ff00;
						
						blue = ( oldColor & 0x000000ff ) + scaledShiftedMaskedBlue;
						if ( blue > 0x000000ff )
							blue = 0x000000ff;
						
						newColor = red | green | blue;
						}
					else
						newColor = 0x00ffffff;
					}
				else
					{
					if ( oldColor )	// if != 0x00000000 (fully saturated black)
						{
						red = ( oldColor & 0x00ff0000 ) - scaledShiftedMaskedRed;
						if ( red < 0x0000ffff )
							red = 0x00000000;
						
						green = ( oldColor & 0x0000ff00 ) - scaledShiftedMaskedGreen;
						if ( green < 0x000000ff )
							green = 0x00000000;
						
						blue = ( oldColor & 0x000000ff ) - scaledShiftedMaskedBlue;
						if ( blue < 0 )
							blue = 0x00000000;
						
						newColor = red | green | blue;
						}
					else
						newColor = 0x00000000;
					}
				if ( newColor != oldColor )
					* reinterpret_cast<int32_t *>( p ) = newColor;
				}
			}
		}
}


void
qdClearLightmap( const RGBColor& moonlight, const RGBColor& starlight )
{
// keep here, or make external, so it can be deleted in deleteNonOGLObjects() ?
// we use this circle a lot, so we'll give it its own optimal-size array
// and eliminate the index scaling
	
	static bool firstTime = true;
	const int circleSize = 326;
	static uchar circularScale[circleSize][circleSize];
	if ( firstTime )
		{
		for ( int x = 0; x < circleSize; ++x )
			{
			for ( int y = 0; y < circleSize; ++y )
				{
				float xPixelCenter = x + 0.5f;
				float yPixelCenter = y + 0.5f;
				float dist = hypotf( xPixelCenter, yPixelCenter );
				if ( dist > circleSize )
					circularScale[y][x] = 0;
				else
					circularScale[y][x] = 0xff * (circleSize - dist) / circleSize;
				}
			}
		firstTime = false;
		}
	
	static int circularScaleColored[circleSize][circleSize];
	static RGBColor oldMoonlight = { 0x0000, 0x0000, 0x0000 };
	static RGBColor oldStarlight = { 0x0000, 0x0000, 0x0000 };
	bool colorChanged = false;
	
	if ( oldMoonlight.red != moonlight.red
	||   oldMoonlight.green != moonlight.green
	||   oldMoonlight.blue != moonlight.blue
	||   oldStarlight.red != starlight.red
	||   oldStarlight.green != starlight.green
	||   oldStarlight.blue != starlight.blue )
		{
		colorChanged = true;
		oldMoonlight = moonlight;
		oldStarlight = starlight;
		}
	
	PixMapHandle pmh = ::GetGWorldPixMap( gLightmapGWorld );
	Ptr baseAddr = ::GetPixBaseAddr( pmh );
	
	// carbon + 8.5; will this be a problem?
	int rowBytes = ::GetPixRowBytes( pmh ) & 0x3fff;
	
	const int bytesPerPixel = 4;
	
	int startH = gLayout.layoFieldBox.rectLeft;
	int endH = gLayout.layoFieldBox.rectRight;
	int startV = gLayout.layoFieldBox.rectTop;
	int endV = gLayout.layoFieldBox.rectBottom;
	
	int centerH = gFieldCenter.ptH;
	int centerV = gFieldCenter.ptV;
	
	int moonlightRed = ( moonlight.red << 8 ) & 0x00ff0000;
	int moonlightGreen = moonlight.green & 0x0000ff00;
	int moonlightBlue = moonlight.blue  >> 8;
	
// trading less computation for more cache thrash was a win
	
	for ( int v = centerV; v < endV; ++v )
		{
		int dv = centerV > v ? centerV - v : v - centerV;
		
		for ( int h = centerH; h < endH; ++h )
			{
			int dh = centerH > h ? centerH - h : h - centerH;
			
			if ( colorChanged )
				{
					// do I need a sanity check to prevent out-of-bounds?
				int colorScale = circularScale[dv][dh];
				int newColor;
				if ( colorScale )
					{
					int red = moonlightRed + ( starlight.red * colorScale ) & 0x00ff0000;
					if ( red > 0x00ff0000 )
						red = 0x00ff0000;
					
					int green = moonlightGreen
						+ ( ( starlight.green * colorScale ) >> 8 ) & 0x0000ff00;
					if ( green > 0x0000ff00 )
						green = 0x0000ff00;
					
					int blue = moonlightBlue
						+ ( ( starlight.blue * colorScale ) >> 16 ) & 0x000000ff;
					if ( blue > 0x000000ff )
						blue = 0x000000ff;
					
					newColor = red | green | blue;
					}
				else
					newColor = moonlightRed + moonlightGreen + moonlightBlue;
				
				char * ll = baseAddr + v * rowBytes
						  + ( startH + endH - h - 1 ) * bytesPerPixel;
				* reinterpret_cast<int32_t *>( ll ) = newColor;
				char * lr = baseAddr + v * rowBytes + h * bytesPerPixel;
				* reinterpret_cast<int32_t *>( lr ) = newColor;
				char * ul = baseAddr
						  + ( startV + endV - v - 1 ) * rowBytes
						  + ( startH + endH - h - 1 ) * bytesPerPixel;
				* reinterpret_cast<int32_t *>( ul ) = newColor;
				char * ur = baseAddr + ( startV + endV - v - 1 ) * rowBytes
						  + h * bytesPerPixel;
				* reinterpret_cast<int32_t *>( ur ) = newColor;
				
				circularScaleColored[dv][dh] = newColor;
				}
			else
				{
				int newColor = circularScaleColored[dv][dh];
				char * ll = baseAddr + v * rowBytes
						  + ( startH + endH - h - 1 ) * bytesPerPixel;
				* reinterpret_cast<int32_t *>( ll ) = newColor;
				char * lr = baseAddr + v * rowBytes + h * bytesPerPixel;
				* reinterpret_cast<long *>( lr ) = newColor;
				char * ul = baseAddr + ( startV + endV - v - 1 ) * rowBytes
						  + ( startH + endH - h - 1 ) * bytesPerPixel;
				* reinterpret_cast<long *>( ul ) = newColor;
				char * ur = baseAddr + ( startV + endV - v - 1 ) * rowBytes
						  + h * bytesPerPixel;
				* reinterpret_cast<int32_t *>( ur ) = newColor;
				}
			}
		}
}


void
qdApplyLightmap()
{
// also buried deep in the gl initialization
// to do: find a better (common, once-only) place for this
	if ( not gIndexToRedMap )
		{
		GetAppData( 'BTbl', 5,
			reinterpret_cast< void** >( &gIndexToRedMap ), nullptr );
		GetAppData( 'BTbl', 6,
			reinterpret_cast< void** >( &gIndexToGreenMap ), nullptr );
		GetAppData( 'BTbl', 7,
			reinterpret_cast< void** >( &gIndexToBlueMap ), nullptr );
		}
#if 0
	CGrafPtr oldCGrafPtr;
	GDHandle oldGDHandle;
	::GetGWorld( &oldCGrafPtr, &oldGDHandle );
	PixMapHandle pmh = ::GetGWorldPixMap ( oldCGrafPtr );
	Ptr baseAddr = ::GetPixBaseAddr( pmh );
	int rowBytes = ::GetPixRowBytes( pmh ) & 0x3fff;
#else
	CLWindow * win = static_cast<CLWindow *>( gGameWindow );
	void * baseAddr = win->winOffView.offBufferImage.GetBits();	// returns baseAddr
	int rowBytes = win->winOffView.offBufferImage.GetRowBytes();
#endif
	
	PixMapHandle pmh = ::GetGWorldPixMap( gLightmapGWorld );
	::LockPixels( pmh );
	Ptr lightmapBaseAddr = ::GetPixBaseAddr( pmh );
	int lightmapRowBytes = ::GetPixRowBytes( pmh ) & 0x3fff;
	
	const int bytesPerPixel = 4;
	
	int startH = gLayout.layoFieldBox.rectLeft;
	int endH = gLayout.layoFieldBox.rectRight;
	int startV = gLayout.layoFieldBox.rectTop;
	int endV = gLayout.layoFieldBox.rectBottom;
	
	static uchar * gRGB444ToIndexMap = nullptr;
		// RGB444 is 4 bits each of red, green, blue; 4096 possible combinations
	if ( not gRGB444ToIndexMap )
		{
#if 1	// resource vs make it ourselves
		GetAppData( 'BTbl', 8,
			reinterpret_cast<void **>( &gRGB444ToIndexMap ), nullptr );
#else
		// pixel needs to be in bounds
		uchar * address = reinterpret_cast<uchar *>( baseAddr )
						+ startV * rowBytes + startH;
		gRGB444ToIndexMap = NEW_TAG("gRGB444ToIndexMap") uchar[16*16*16];
		if ( not gRGB444ToIndexMap )
			return;
		
		for ( ushort r = 0; r < 16; ++r )
			{
			for ( ushort g = 0; g < 16; ++g )
				{
				for ( ushort b = 0; b < 16; ++b )
					{
					RGBColor c = { r << 12, g << 12, b << 12 };
					::SetCPixel( startH, startV, &c );
					ushort index = ( r << 8 ) + ( g << 4 ) + b;
					gRGB444ToIndexMap[ index ] = *address;
#if 1	// confidence test
					int i2cIndex = ::Color2Index( &c );
						// the docs say we should avoid using Color2Index
					if ( i2cIndex != gRGB444ToIndexMap[ index ] )
						{
						ShowMessage( "unexpected color mismatch when "
							"generating gRGB444ToIndexMap: "
							"%d != %d (%x, %x, %x(",
							i2cIndex, gRGB444ToIndexMap[ index ],
							c.red, c.green, c.blue );
//						gRGB444ToIndexMap[ index ] = i2cIndex;
						}
#endif	// confidence test
					}
				}
			}
#if 1	// confidence test
			// make sure the original colors come through true
		for ( int index = 0; index < 256; ++index )
			{
			ushort r = gIndexToRedMap[ index ];
			ushort g = gIndexToGreenMap[ index ];
			ushort b = gIndexToBlueMap[ index ];
			
			ulong rgb444 = ( ( r & 0xf000 ) >> 4 )
						+ ( ( g & 0xf000 ) >> 8 ) + ( ( b & 0xf000 ) >> 12 );
			
			if ( gRGB444ToIndexMap[ rgb444 ] != index )
				{
				ShowMessage( "mismatch @ %u (%u): %x %x %x != %x %x %x", index, rgb444,
				gIndexToRedMap[ gRGB444ToIndexMap[ rgb444 ] ],
				gIndexToGreenMap[ gRGB444ToIndexMap[ rgb444 ] ],
				gIndexToBlueMap[ gRGB444ToIndexMap[ rgb444 ] ],
				r, g, b );
				}
//			gRGB444ToIndexMap[ rgb444 ] = index;
			}
#endif	// confidence test

// barf out the data in rez format so we can make a proper resource out of it
// (is there a way to suppress the timestamp?)
		for ( int i = 0;  i < 16*16*16;  i += 16 )
			{
			ShowMessage( "\t$\"%2.2x %2.2x %2.2x %2.2x %2.2x %2.2x %2.2x %2.2x "
							  "%2.2x %2.2x %2.2x %2.2x %2.2x %2.2x %2.2x %2.2x\"",
				gRGB444ToIndexMap[i+ 0], gRGB444ToIndexMap[i+ 1],
				gRGB444ToIndexMap[i+ 2], gRGB444ToIndexMap[i+ 3],
				gRGB444ToIndexMap[i+ 4], gRGB444ToIndexMap[i+ 5],
				gRGB444ToIndexMap[i+ 6], gRGB444ToIndexMap[i+ 7],
				gRGB444ToIndexMap[i+ 8], gRGB444ToIndexMap[i+ 9],
				gRGB444ToIndexMap[i+10], gRGB444ToIndexMap[i+11],
				gRGB444ToIndexMap[i+12], gRGB444ToIndexMap[i+13],
				gRGB444ToIndexMap[i+14], gRGB444ToIndexMap[i+15]
			);
			}
#endif	// resource switch
		}
	
	for ( int v = startV; v < endV; ++v )
		{
		for ( int h = startH; h < endH; ++h )
			{
				// could be slightly faster (use add instead of multiply)
			ulong * lightmapPtr = reinterpret_cast<ulong *>(
					 lightmapBaseAddr + v * lightmapRowBytes + h * bytesPerPixel );
			ulong lightmapColor = * lightmapPtr;
			
				// could be slightly faster (use add instead of multiply)
			uchar * destPtr = reinterpret_cast<uchar *>( baseAddr )
							+ v * rowBytes + h;
			uchar destColorIndex = * destPtr;
			
				// these four lines could be combined...
			ushort red   = ( ( ( lightmapColor & 0x00ff0000 ) >> 8 )
							* gIndexToRedMap  [destColorIndex] ) >> 16;
			ushort green = (   ( lightmapColor & 0x0000ff00 )
							* gIndexToGreenMap[destColorIndex] ) >> 16;
			ushort blue  = ( ( ( lightmapColor & 0x000000ff ) << 8 )
							* gIndexToBlueMap [destColorIndex] ) >> 16;
			
			ushort index = ( ( red & 0xf000 ) >> 4 )
						 + ( ( green & 0xf000 ) >> 8 )
						 + ( ( blue & 0xf000 ) >> 12 );
			* destPtr = gRGB444ToIndexMap[ index ];
				// could do some black magic and move 8 bytes per write...
			}
		}
	::UnlockPixels( ::GetGWorldPixMap ( gLightmapGWorld ) );
}


void
qdClampLightmap( float nightCLampColor )
{
	PixMapHandle pmh = ::GetGWorldPixMap ( gLightmapGWorld );
	::LockPixels( pmh );
	Ptr lightmapBaseAddr = ::GetPixBaseAddr( pmh );
	int lightmapRowBytes = ::GetPixRowBytes( pmh ) & 0x3fff;	// carbon + 8.5
	
	const int bytesPerPixel = 4;
	
	int startH = gLayout.layoFieldBox.rectLeft;
	int endH = gLayout.layoFieldBox.rectRight;
	int startV = gLayout.layoFieldBox.rectTop;
	int endV = gLayout.layoFieldBox.rectBottom;
	
	uint nightCLampBlue = nightCLampColor * 0xff;
	uint nightCLampGreen = nightCLampBlue << 8;
	uint nightCLampRed = nightCLampBlue << 16;
	
	for ( int v = startV; v < endV; ++v )
		{
		for ( int h = startH; h < endH; ++h )
			{
			uint32_t * lightmapPtr = reinterpret_cast<uint32_t *>(
				lightmapBaseAddr + v * lightmapRowBytes + h * bytesPerPixel );
			uint32_t lightmapColor = *lightmapPtr;
			
			uint red = lightmapColor & 0x00ff0000;
			if ( red < nightCLampRed )
				red = nightCLampRed;
			uint green = lightmapColor & 0x0000ff00;
			if ( green < nightCLampGreen )
				green = nightCLampGreen;
			uint blue = lightmapColor & 0x000000ff;
			if ( blue < nightCLampBlue )
				blue = nightCLampBlue;
			
			uint32_t color = red | green | blue;
			
			if ( lightmapColor != color )
				*lightmapPtr = color;
			}
		}
	::UnlockPixels( ::GetGWorldPixMap( gLightmapGWorld ) );
}
#endif	// USE_OPENGL


#ifdef TEMP_LIGHTING_DATA
void
FillTempLightingDataStruct( CLImage & image )
{
	enum {
		kLightData_torchlight,
		kLightData_fire,
		kLightData_mysticYellow,
		kLightData_healerBlue,
		kLightData_redPulse,
		kLightData_midWhite,
		kLightData_highCyan,
		kLightData_highPurple,
		kLightData_pureRed,
		kLightData_purePurple,
		kLightData_pureYellow,
		kLightData_pureCyan,
		kLightData_pureBlue,
		kLightData_pureWhite,
		kLightData_lantern,

		kLightData_darkcaster7,
		kLightData_darkcaster15,
		kLightData_darkcaster31,
		kLightData_darkcaster47,
		kLightData_darkcaster63,
		kLightData_darkcaster79,
		kLightData_darkcaster95,
		kLightData_darkcaster111,
		kLightData_darkcaster127,
		kLightData_darkcaster143,
		kLightData_darkcaster159,
		kLightData_darkcaster175,
		kLightData_darkcaster191,
		kLightData_darkcaster207,
		kLightData_darkcaster223,
		kLightData_darkcaster239,
		kLightData_darkcaster255,

			// this gives every exile icon a small pseudo-torch,
			// until custom exile lights are implemented
		kLightData_exileLight,
			// this needs to be removed when we get a better notion of exile lighting
		kLightData_oldNight
	};

	static const LightingData standardLightData[] = {
		{ { 255, 255, 255, 255 },   0,   0 },		// kLightData_torchlight
//		{ { 255,  63,  31, 255 },   0,   0 },		// kLightData_fire
		{ { 255, 191, 127, 255 },   0,   0 },		// kLightData_fire
// with exile pseudo-torches, we can remove the "white" component of these healer/mystic pulses
//		{ {  95,  95,  31, 255 },   0,   5 },		// kLightData_mysticYellow
//		{ {  63,  63,   0, 255 },   0,   5 },		// kLightData_mysticYellow
		{ { 255, 255,   0, 255 },   0,   5 },		// kLightData_mysticYellow
//		{ {  31,  31,  95, 255 },   0,   5 },		// kLightData_healerBlue
//		{ {   0,   0,  63, 255 },   0,   5 },		// kLightData_healerBlue
		{ {   0,   0, 255, 255 },   0,   5 },		// kLightData_healerBlue
		{ {  95,  31,  31, 255 },   0,   0 },		// kLightData_redPulse
		{ { 127, 127, 127, 255 },   0,   0 },		// kLightData_midWhite
		{ { 127, 255, 255, 255 },   0,   0 },		// kLightData_highCyan
		{ { 255, 127, 255, 255 },   0,   0 },		// kLightData_highPurple
		{ { 255,   0,   0, 255 },   0,   0 },		// kLightData_pureRed
		{ { 255,   0, 255, 255 },   0,   0 },		// kLightData_purePurple
		{ { 255, 255,   0, 255 },   0,   0 },		// kLightData_pureYellow
		{ {   0, 255, 255, 255 },   0,   0 },		// kLightData_pureCyan
		{ {   0,   0, 255, 255 },   0,   0 },		// kLightData_pureBlue
		{ { 255, 255, 255, 255 },   0,   0 },		// kLightData_pureWhite
		{ { 255, 255, 255, 255 }, 100,   0 },		// kLightData_lantern

		{ {   7,   7,   7, 255 },   0,   7 },		// kLightData_darkcaster7
		{ {  15,  15,  15, 255 },   0,  15 },		// kLightData_darkcaster15
		{ {  31,  31,  31, 255 },   0,  31 },		// kLightData_darkcaster31
		{ {  47,  47,  47, 255 },   0,  47 },		// kLightData_darkcaster47
		{ {  63,  63,  63, 255 },   0,  63 },		// kLightData_darkcaster63
		{ {  79,  79,  79, 255 },   0,  79 },		// kLightData_darkcaster79
		{ {  95,  95,  95, 255 },   0,  95 },		// kLightData_darkcaster95
		{ { 111, 111, 111, 255 },   0, 111 },		// kLightData_darkcaster111
		{ { 127, 127, 127, 255 },   0, 127 },		// kLightData_darkcaster127
		{ { 143, 143, 143, 255 },   0, 143 },		// kLightData_darkcaster143
		{ { 159, 159, 159, 255 },   0, 159 },		// kLightData_darkcaster159
		{ { 175, 175, 175, 255 },   0, 175 },		// kLightData_darkcaster175
		{ { 191, 191, 191, 255 },   0, 191 },		// kLightData_darkcaster191
		{ { 207, 207, 207, 255 },   0, 207 },		// kLightData_darkcaster207
		{ { 223, 223, 223, 255 },   0, 223 },		// kLightData_darkcaster223
		{ { 239, 239, 239, 255 },   0, 239 },		// kLightData_darkcaster239
		{ { 255, 255, 255, 255 },   0, 255 },		// kLightData_darkcaster255

		{ { 127, 127, 127, 255 },   0, 255 },		// kLightData_exileLight
		{ { 127, 127, 127, 255 }, 325,   0 }		// kLightData_oldNight	// safe to remove?
	};

	if ( image.cliPictDef.pdFlags & kPictDefFlagEmitsLight )
		{
#if defined(USE_OPENGL) && defined(DEBUG_VERSION) && 1
		ShowMessage("image id %d already has lighting date: remove from override switch",
			image.cliPictDefID );
#endif	// USE_OPENGL && DEBUG_VERSION
		}
	else
		{

	switch( image.cliPictDefID )
		{
//		case 2227:	// spear
//		case 2230:	// arrow
			// these appear to be flaming in some poses.  with the old
			// way, we just checked for the correct pose in the big ugly
			// switch. With the new way, there is no way to do that
			// without reintroducing the big ugly switch for the special
			// cases. suggestion:  create separate "flaming arrow" and
			// "flaming spear" images, which makes the image file
			// slightly bigger, but simplifies the drawing code by
			// avoiding the switch.
		
		case 32:	// altar
		case 318:	// brazier
		case 330:	// torch
		case 331:	// torch
		case 2460:	// torch
		case 2461:	// torch
		case 2478:	// altar
		case 3212:	// fireplace
		case 3331:	// torch
		case 3332:	// torch
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_torchlight];
			break;

		// attack-only mobiles

		case 2092:	// ??
		case 2573:	// ?? drake
		case 1406:	// jawa
		case 1344:	// electric eel
		case 629:	// will o' wisp
		case 421:	// ??
		case 157:	// ??
			image.cliPictDef.pdFlags |= kPictDefFlagOnlyAttackPosesLit;
			FALLTHRU_OK;
		
		case 40:	// ??
		case 799:	// glowy circle thing
		case 1226:	// ??
		case 1294:	// ??
		case 1401:	// ??
		case 1407:	// ??
		case 1494:	// ??
		case 1570:	// ??
		case 1571:	// ??
		case 1587:	// ??
		case 1588:	// ??
		case 1589:	// ??
		case 1590:	// ??
		case 1591:	// ??
		case 1592:	// ??
		case 1593:	// ??
		case 1594:	// ??
		case 1644:	// pumpkin
		case 1710:	// altar
		case 1762:	// fancy portal/mirror
		case 1774:	// candle
		case 1789:	// ??
		case 1830:	// xmas wreath
		case 1831:	// xmas wreath
		case 1836:	// bday cake
		case 1837:	// decorations
		case 1838:	// decorations
		case 1895:	// rainbow lightning	// these USED to change color every frame.
		case 1896:	// rainbow lightning	// to do that with the new scheme, we would
		case 1897:	// rainbow lightning	// need yet another flag, and I'm not sure
		case 1898:	// rainbow lightning	// it's worth it.
		case 1899:	// rainbow lightning	// alternately(1), the server could send one frame
		case 1900:	// rainbow lightning	// selected randomly from the existing lightnings
		case 1901:	// rainbow lightning	// and achieve the same effect
		
						// alternately(2), since kPictDefFlagOnlyAttackPosesLit
						// is only used for mobiles, we could reuse that bit for
						// non-mobiles to mean "randomly cycle through various
						// primary and secondary colors"
		case 2052:	// ??
		case 2299:	// lamp
											// these three need more attention...
		case 2313:	// alternate light??
		case 2314:	// alternate light??
		case 2315:	// alternate light??
		case 2332:	// prelit stone cat
		case 2458:	// ??
		case 2507:	// xmas lights
		case 2901:	// rift
		case 2960:	// ??
		case 2976:	// ??
		case 2977:	// ??
		case 2978:	// ??
			// ??
		case 3093: case 3094: case 3095: case 3096: case 3097:
		case 3098: case 3099: case 3100: case 3101: case 3102:
		case 3103: case 3104: case 3105: case 3106: case 3107:
		case 3108: case 3109: case 3110: case 3111: case 3112:
		case 3113: case 3114: case 3115: case 3116: case 3117:
		case 3118: case 3119: case 3120: case 3121: case 3122: case 3123: case 3124:

		case 3126:	// ??
		case 3400:	// ??
		case 3401:	// ??
		case 1628:	// pumpkin
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_pureWhite];
			break;

		// attack-only mobiles

//		case 159:	// ??
//			image.cliPictDef.pdFlags |= kPictDefFlagOnlyAttackPosesLit;

		case 445:	// mystic boost
		case 446:	// mystic boost
		case 1286:	// mystic boost afterglow
		case 1729:	// ??
		case 1730:	// ??
		case 1731:	// ??
		case 1732:	// ??
		case 1733:	// orb burst
		case 1734:	// ??
		case 1735:	// ??
		case 1736:	// ??
		case 1737:	// ??
		case 1738:	// ??
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_mysticYellow];
			break;

		// attack-only mobiles

		case 2024:	// fire bird
		case 1691:	// fire drake
		case 1701:	// fire drake
		case 1431:	// fire-breathing ?
		case 1327:	// fire walker
					// this one ought to be moved to darkcaster
//		case 1270:	// skull
		case 817:	// fire-breathing ?
		case 689:	// fire-breathing ?
		case 590:	// fire-breathing ?
		case 501:	// fire drake
		case 506:	// forest drake
		case 288:	// ??
			image.cliPictDef.pdFlags |= kPictDefFlagOnlyAttackPosesLit;
			FALLTHRU_OK;
		
		case 481:	// flames
		case 482:	// flames
		case 572:	// flames
		case 597:	// lava pool
		case 598:	// lava pool
		case 679:	// flames
		case 680:	// flames
		case 681:	// flames
		case 969:	// lava pool
		case 1567:	// flames
		case 1568:	// flames
		case 1569:	// flames
		case 2078:	// static firewalker??
		case 2087:	// extinguished fire
		case 2090:	// circle o' fire
		case 2219:	// flaming hampster?
		case 2220:	// flaming hampster?
		case 2221:	// flaming hampster?
		case 2222:	// flaming hampster?
		case 2238:	// flaming chasm
		case 2239:	// flaming chasm
		case 2256:	// lava
		case 2257:	// lava
		case 2258:	// lava
		case 2259:	// lava
		case 2260:	// lava
		case 2261:	// lava
		case 2262:	// lava
		case 2263:	// lava
		case 2264:	// lava
		case 2607:	// flames
		case 2608:	// flames
		case 2664:	// flames
		case 3025:	// flames
		
		case 2728:	// fire beatle
		case 2729:	// fire beatle
		case 2771:	// cannonball

			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_fire];
			break;

		case 579:	// red lightning
		case 580:	// red lightning
		case 581:	// red lightning
		case 582:	// red lightning
		case 583:	// red lightning
		case 584:	// red lightning
		case 585:	// red lightning
		case 587:	// red lightning
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_pureRed];
			break;

		case 1191:	// rift
		case 1192:	// rift
		case 1193:	// rift
		case 1759:	// healer blue?
		case 1760:	// healer blue?
		case 2988:	// blue pulse
		case 2989:	// blue pulse
		case 3504:	// shared healer blue?
		case 3505:	// shared healer blue?
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_healerBlue];
			break;

		// attack-only mobiles

					// this one ought to be moved to darkcaster
//		case 1970:	// skull
//			image.cliPictDef.pdFlags |= kPictDefFlagOnlyAttackPosesLit;

		case 1356:	// colored lightning
		case 1357:	// colored lightning
		case 1358:	// colored lightning
		case 1359:	// colored lightning
		case 1360:	// colored lightning
		case 1361:	// colored lightning
		case 1362:	// colored lightning
		case 1363:	// colored lightning
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_highCyan];
			break;

		case 1739:	// odd flames
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_purePurple];
			break;

		case 1740:	// odd flames
		case 2364:	// yellow lightning
		case 2365:	// yellow lightning
		case 2366:	// yellow lightning
		case 2367:	// yellow lightning
		case 2368:	// yellow lightning
		case 2369:	// yellow lightning
		case 2370:	// yellow lightning
		case 2371:	// yellow lightning
		case 2936:	// dark yellow lightning
		case 2937:	// dark yellow lightning
		case 2938:	// dark yellow lightning
		case 2939:	// dark yellow lightning
		case 2940:	// dark yellow lightning
		case 2941:	// dark yellow lightning
		case 2942:	// dark yellow lightning
		case 2943:	// dark yellow lightning
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_pureYellow];
			break;

		case 1741:	// odd flames
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_pureCyan];
			break;

		case 1888:	// lantern
		case 1889:	// lantern
		case 1890:	// lantern
		case 1925:	// lantern
		case 1926:	// lantern
		case 1927:	// lantern
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_lantern];
			break;

		case 2725:	// blue flames
		case 2958:	// blue flames
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_pureBlue];
			break;

		case 2794:	// pink lightning
		case 2795:	// pink lightning
		case 2796:	// pink lightning
		case 2797:	// pink lightning
		case 2798:	// pink lightning
		case 2799:	// pink lightning
		case 2800:	// pink lightning
		case 2801:	// pink lightning
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_highPurple];
			break;

		case 3125:	// red pulse
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_redPulse];
			break;

		// attack-only mobiles

		case 568:	// noth
		case 648:	// ??
		case 1969:	// noth
		case 1973:	// noth
		case 1978:	// noth
		case 1979:	// noth
		case 1980:	// noth
		case 1981:	// noth
		case 1982:	// noth
		case 1983:	// noth
		case 1984:	// noth
			image.cliPictDef.pdFlags |= kPictDefFlagOnlyAttackPosesLit;

			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_midWhite];
			break;

#if 1
	// probably best if these DON'T get put into image file,
	// until the details of per-exile lighting are sorted out
			// various exiles
		case 5:
		case 21:
		case 22:
		case 25:	// peasant female
		case 28:	// boat rental
		case 53:
		case 54:
		case 62:
		case 63:
		case 64:
		case 65:
		case 76:	// peasant male
		case 131:
		case 158:	// gaia
		case 159:	// male trainer
		case 537:	// boat
		case 447:
		case 448:
		case 449:
		case 450:
		case 451:
		case 452:
		case 453:
		case 454:
		case 455:
		case 456:
		case 457:
		case 458:
		case 459:
		case 460:
		case 507:
	// "butterfly kiss" -- not sure how (or whether!) to distinguish players
	// from plain old butterfly critters...
	//
	// jrr - how: duplicate the butterfly icon.  player version gets light date,
	//            critter version does not (this requires a server/script tweak
	//            to mechanize)
		case 973: case 1084: case 1085: case 1086: case 1087:
		case 1089: case 1090: case 1092: case 1096:
		case 1261:	// motley fool
		case 1262:
		case 1309:	// female trainer
		case 1426:
		case 1583:
		case 1784:	// exile-turned-undine in undine hut...
					//  lightcaster?  darkcaster?  is it ever used as a monster?
		case 1839:
		case 1840:
		case 1841:
		case 1902:	// rebel knights
		case 1903:
		case 1904:
		case 1934:
		case 2166:
		case 2399:
		case 2400:
		case 2468:
		case 2791:
		case 2951:
		case 2999:
		case 3000:
		case 3001:
		case 3002:
		case 3003:
		case 3004:
		case 3005:
		case 3006:
		case 3007:
		case 3008:
		case 3009:
		case 3010:
		case 3011:
		case 3012:
		case 3013:
		case 3014:
		case 3324:
		case 3325:
		case 3355:
		case 3356:
		case 3426:	// mystic/attache robes
		case 3427:	// mystic/attache robes
		case 3456:
		case 3665:
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliLightInfo = standardLightData[kLightData_exileLight];
			break;
#endif  // 1

#if 0
	// testing done, i think.  disabled, so the GMs can decide where (or if)
	// it will actually get used
	
// some darkcasters.  undine were chosen for testing, but likely won't stay this way
		case 88:	// skeletal
		case 231:	// giant skeletal
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliPictDef.pdFlags |= kPictDefFlagLightDarkcaster;
			image.cliLightInfo = standardLightData[kLightData_darkcaster7];
			break;

		case 236:	// weak undine
		case 237:
		case 238:
		case 239:
		case 240:
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliPictDef.pdFlags |= kPictDefFlagLightDarkcaster;
			image.cliLightInfo = standardLightData[kLightData_darkcaster7];
			break;

		case 82:	// ghost
		case 312:	// spirits
		case 619:	// spirits
		case 620:	// spirits
		case 388:	// spirits
		case 389:	// spirits
		case 385:	// spirits
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliPictDef.pdFlags |= kPictDefFlagLightDarkcaster;
			image.cliLightInfo = standardLightData[kLightData_darkcaster7];
			break;

		case 219:	// wraith
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliPictDef.pdFlags |= kPictDefFlagLightDarkcaster;
			image.cliLightInfo = standardLightData[kLightData_darkcaster7];
			break;

		case 77:	// liche
			image.cliPictDef.pdFlags |= kPictDefFlagEmitsLight;
			image.cliPictDef.pdFlags |= kPictDefFlagLightDarkcaster;
			image.cliLightInfo = standardLightData[kLightData_darkcaster127];
			break;
#endif  // 0

		default:
			break;
		}

	switch ( image.cliPictDefID )
		{
		case 32:	// altar
		case 318:	// brazier
		case 330:	// torch
		case 331:	// torch
		case 2460:	// torch
		case 2461:	// torch
		case 2478:	// altar
		case 3212:	// fireplace
		case 3331:	// torch
		case 3332:	// torch
		case 2024:	// fire bird
		case 1691:	// fire drake
		case 1701:	// fire drake
		case 1431:	// fire-breathing ?
		case 1327:	// fire walker
		case 817:	// fire-breathing ?
		case 689:	// fire-breathing ?
		case 590:	// fire-breathing ?
		case 501:	// fire drake
		case 506:	// forest drake
		case 288:	// ??
		case 481:	// flames
		case 482:	// flames
		case 572:	// flames
		case 679:	// flames
		case 680:	// flames
		case 681:	// flames
		case 1567:	// flames
		case 1568:	// flames
		case 1569:	// flames
		case 2078:	// static firewalker??
		case 2087:	// extinguished fire
		case 2090:	// circle o' fire
		case 2219:	// flaming hampster?
		case 2220:	// flaming hampster?
		case 2221:	// flaming hampster?
		case 2222:	// flaming hampster?
		case 2607:	// flames
		case 2608:	// flames
		case 2664:	// flames
		case 3025:	// flames
			image.cliPictDef.pdFlags |= kPictDefFlagLightFlicker;
		};
	}	//  override bypass
	
	if ( image.cliPictDef.pdFlags & kPictDefFlagRandomAnimation )
		{
#if defined(USE_OPENGL) && defined(DEBUG_VERSION)
		ShowMessage( "image id %d already has kPictDefFlagRandomAnimation: "
					"remove from override switch", image.cliPictDefID );
#endif	// USE_OPENGL && DEBUG_VERSION
		}
	else
	switch ( image.cliPictDefID )
		{
		case 579: case 580: case 581: case 582:
		case 583: case 584: case 585: case 587:			// red lightning
		
		case 1356: case 1357: case 1358: case 1359:
		case 1360: case 1361: case 1362: case 1363:		// colored lightning
		
		case 1895: case 1896: case 1897: case 1898:
		case 1899: case 1900: case 1901:				// rainbow lightning
		
		case 2364: case 2365: case 2366: case 2367:
		case 2368: case 2369: case 2370: case 2371:		// yellow lightning
		
		case 2794: case 2795: case 2796: case 2797:
		case 2798: case 2799: case 2800: case 2801:		// pink lightning
		
		case 2936: case 2937: case 2938: case 2939:
		case 2940: case 2941: case 2942: case 2943:		// dark yellow lightning

			image.cliPictDef.pdFlags |= kPictDefFlagRandomAnimation;
		};
}
#endif	// TEMP_LIGHTING_DATA

