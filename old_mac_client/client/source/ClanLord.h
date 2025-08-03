/*
**	ClanLord.h		ClanLord
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

#ifndef CLANLORD_H
#define	CLANLORD_H

// MacRoman bullet character (needed by TranslatableTextDefs.h)
#define BULLET "\xA5"


// ubiquitous includes
#include "DatabaseTypes_cl.h"
#include "Public_cl.h"

#ifndef TRANSLATABLE_TEXT_DEFS_H
# include "TranslatableTextDefs.h"
#endif


/*
**	Definitions
*/
using std::snprintf;

// turn this on to collect statistics on image usage in incoming data
#if 0
# if defined( DEBUG_VERSION ) && ! defined( DEBUG_IMAGECOUNT )
#  define DEBUG_IMAGECOUNT		1
# endif
#endif // 0

// turn this on for colored text in info pane and text window
#ifndef USE_STYLED_TEXT
# define USE_STYLED_TEXT		1
#endif

#include "StyledTextField_cl.h"

#if USE_STYLED_TEXT
// JEB turn this on for clickable hyperlinks in the text window and info pane
# define TextFieldType			URLStyledTextField
#else
# define TextFieldType			DTSTextField
#endif

// JEB turn this on to use a macro folder
#ifndef USE_MACRO_FOLDER
# define USE_MACRO_FOLDER		1
#endif

// turn this on for text-to-speech
#ifndef USE_SPEECH
# define USE_SPEECH				1
#endif

// JEB Uncomment for client commands
#ifndef CLIENT_COMMANDS
# define CLIENT_COMMANDS		1
#endif

// turn this on to enable support for fadeaway nametags
#define AUTO_HIDENAME			1

// JRR turn this on for whisper/yell in languages
#ifndef USE_WHISPER_YELL_LANGUAGE
# define USE_WHISPER_YELL_LANGUAGE	1
#endif

// JRR turn this on to make lightning more irregular
#define IRREGULAR_ANIMATIONS	1

// this should be in DTS.h but isn't for some reason
const int kResultCancel		= 1;

// background music player -- disabled; please use iTunes instead
#define ENABLE_MUSIC_FILES		0


/*
**	resource and menu IDs
*/
#include "ResourceIDs.h"


/*
**	Friendship classes
*/
enum
{
	kFriendAmbiguous	= -2,	// error code: given label name is ambiguous
	kFriendNotFound		= -1,	// error code: given label name not found
	kFriendNone			= 0,	// nothing special
	kFriendLabel1		= 1,	// has label 1 (red)
	kFriendLabel2		= 2,	// has label 2 (orange)
	kFriendLabel3		= 3,	// has label 3 (green)
	kFriendLabel4		= 4,	// has label 4 (blue)
	kFriendLabel5		= 5,	// has label 5 (purple)
	kFriendBlock		= 6,	// is being silenced
	kFriendIgnore		= 7,	// is being totally ignored
	kFriendLabelLast	= kFriendLabel5,
	kFriendFlagCount 	= kFriendIgnore + 1
};


/*
** Info text types
*/
enum MsgClasses
{
	kMsgDefault,		// 0
	kMsgLogon,
	kMsgFriendLogon,
	kMsgLogoff,
	kMsgFriendLogoff,
	kMsgShare,			// 5
	kMsgFriendShare,
	kMsgHost,
	kMsgObit,
	kMsgFriendObit,
	kMsgSpeech,			// 10
	kMsgFriendSpeech,
	kMsgMySpeech,
	kMsgBubble,
	kMsgFriendBubble,
	kMsgThoughtMsg,
	kMsgTimeStamp,
	kMsgLastMsg,		// must be the last in this series
	
	kMsgWho,			// 18 no prefs for these
	kMsgInfo,
	kMsgBardLevel,
	kMsgDownload,
	kMsgMonoStyle,
	
	kMsgMaxStyles	= 32		// room to grow
};


enum BEPPrefixID
{
	kBEEPBardMessageID		= '\302ba ',	// \302 is \xC2 is opt-l, U+00AC "not sign"
	kBEPPBackEndCmdID		= '\302be ',
	kBEPPClanNameID			= '\302cn ',
	kBEPPConfigID			= '\302cf ',		// new v181.2
	kBEPPDontDisplayID		= '\302dd ',
	kBEPPDemoID				= '\302de ',
	kBEPPDepartID			= '\302dp ',
	kBEPPDownloadID			= '\302dl ',
	kBEPPErrID				= '\302er ',
	kBEPPGameMasterID		= '\302gm ',
	kBEPPHasFallenID		= '\302hf ',
	kBEPPNoLongerFallenID	= '\302nf ',		// new v252
	kBEPPHelpID				= '\302hp ',
	kBEPPInfoID				= '\302in ',
	kBEPPInventoryID		= '\302iv ',
	kBEPPKarmaID			= '\302ka ',
	kBEPPKarmaReceivedID	= '\302kr ',		// new for v204
	kBEPPLogOffID			= '\302lf ',		// new v157.1
	kBEPPLogOnID			= '\302lg ',		// new meaning v157.1
	kBEPPLogOnOffID			= '\302lg ',		// deprecated v155+
	kBEPPLocationID			= '\302lo ',
	kBEPPMonsterNameID		= '\302mn ',
#ifdef MULTILINGUAL
	kBEPPMultiLingualID		= '\302ml ',
#endif // MULTILINGAL
	kBEPPMusicID			= '\302mu ',
	kBEPPNewsID				= '\302nw ',
	kBEPPPlayerNameID		= '\302pn ',
	kBEPPShareID			= '\302sh ',
	kBEPPUnshareID			= '\302su ',		// new v157.1
	kBEPPTextLogOnlyID		= '\302tl ',
	kBEPPThinkID			= '\302th ',
	kBEPPMonoStyleID		= '\302tt ',		// new v224
	kBEPPWhoID				= '\302wh ',
	kBEPPYouKilledID		= '\302yk '
};


/*
**	Game Window Definitions
*/
enum
{
	// the margin for the edit text box
	kGameInputMargin		= 3,
	
	// the height & width of the object boxes
	kGameObjectHeight		= 42,
	kGameObjectWidth		= 42,
};


/*
**	standard picture IDs
*/
enum
{
	kSmallBubblePict		= 1,		// speech bubble backgrounds
	kMediumBubblePict		= 2,
	kLargeBubblePict		= 3,
	kEmptyLeftHandPict		= 6,		// client game window shows these if ...
	kEmptyRightHandPict		= 78,		// ... nothing equipped in the corresponding slot
};


#pragma pack( push, 2 )

/*
**	PrefsData
**
**	update kPrefsVersion when you change the PrefsData structure
**	Also, if possible, add the relevant case to UpdatePrefs()
*/
const int kPrefsVersion		= 52;
const int kItemPrefsVersion	= 3;

struct PrefsData
{
	int32_t			pdVersion;					// prefs version number				g + c
	
	DTSRect			pdGWPos;					// game window position				c
	DTSRect			pdTextPos;					// text window position				c
	DTSRect			pdInvenPos;					// inventory window position		c
	DTSRect			pdPlayersPos;				// players window position			c
	DTSPoint		pdMsgPos;					// message window position			c
	
	int32_t			pdShowNames;				// show player/npc names			c
	int32_t			pdGodHideMonsterNames;		// Hide names for monsters			c
	int32_t			pdSound;					// play sounds						c
	int32_t			pdTimeStamp;				// show time in the text window		c
	int32_t			pdLargeWindow;				// use the large window				c
	int32_t			pdBrightColors;				// use brighter background colors.	c
	int32_t			pdMoveControls;				// mouse control setting			c
	int32_t			pdSuppressClanning;			// suppress clanning message.		c
	int32_t			pdSuppressFallen;			// suppress fallen message.			c
	int32_t			pdMusicPlay;				// play music or not				c
	int32_t			pdMusicVolume;				// music volume						c
	
	int32_t			pdSavePassword;				// remember and use the passwords	g
	char			pdAccount [ kMaxNameLen ];	// account name						g
	char			pdAcctPass[ kMaxPassLen ];	// account password					g
	char			pdCharName[ kMaxNameLen ];	// character name					c
	char			pdCharPass[ kMaxPassLen ];	// character password				c
	
	int32_t			pdShowFrameTime;			// debugging options				g
	int32_t			pdSaveMsgLog;				// are debug msgs logged			g
	int32_t			pdSaveTextLog;				// is text win logged				c
	int32_t			pdShowDrawTime;				// show screen draw time			g
	int32_t			pdShowImageCount;			// show # images per frame			g
	int32_t			pdFastBlitters;				// use hispeed blits				g
	char			pdHostAddr[64];				// host ip address					g
	char			pdProxyAddr[64];			// proxy address			 		g
	
	CLStyleRecord	pdStyles[ kMsgMaxStyles ];	// misc text styles					c
	
	int32_t			pdMaxNightPct;				// max %age night displayed			c
	int32_t			pdBubbleBlitter;			// blitter used for normal bubbles	c
	int32_t			pdFriendBubbleBlitter;		// blitter used for friend bubbles	c
	
// v30
	DTSBoolean		pdSpeaking[ kMsgMaxStyles ];	// speak this msg aloud?		c
	
// v31
	int32_t			pdItemPrefsVers;			// version of item-prefs			g
	
// v32
	int32_t			pdHBColorStyle;				// value, type, ...					c
	int32_t			pdHBPlacement;				// center, ll, lr, ur, ...			c
	
// v33
	int32_t			pdThinkNotify;				// notify for /thinkto's			c

// v34
	// no new fields, just reinterpreted the saved passwords
	// they are now SimpleEncrypted on disk

// v35
	int32_t			pdDisableCmdQ;				// mask cmd-q menu key when online

// v36
	int32_t			pdNoSuppressFriends;		// friend messages aren't suppressed

// v38
	int32_t			pdUseArbitrary;				// use an arbitrary client port		g
	int32_t			pdClientPort;				// client port						g
	
// v39											// changed password encryption		g+c

// v40
	int32_t			pdTreatSharesAsFriends;		// added treat-shares-as-friends

// v41	no new fields, but enlarged pdStyles[] and pdSpeaking[]

// v42
	int32_t			pdSoundVolume;				// [0-100] controls CLsounds, not music movies

// v43
	int32_t			pdSurveys;					// bitmap for hardware survey

// v44
	int32_t			pdNumFrames;				// total number of frames played
	int32_t			pdLostFrames;				// number of lost frames
	
// v45
	int32_t			pdClientVersion;			// client version number
	
// v46
	int32_t			pdCompiledClient;			// compiled client version number

// v47
	int32_t			pdOpenGLEnable;				// master enable
	int32_t			pdOpenGLEnableEffects;		// ogl-specific effects, like smooth night
	int32_t			pdOpenGLRenderer;			// 0 = let client pick renderer
												// 1 = force software renderer
												// 2 = allow "non-compliant" hardware
// v48
	int32_t			pdNewLogEveryJoin;			// start a new log file with every join
	int32_t			pdNoMovieTextLogs;			// don't save log files when playing movies

// v49
	int32_t			pdBardVolume;				// 0-100 separate master volume for bard songs

// v50
	int32_t			pdLanguageID;			// initial gRealLangID, if MULTILINGUAL (else ignored)
};

#pragma pack( pop )

// hitherto, the gPrefsData.pdSavePassword boolean did double duty, for both character-
// and account-level passwords. Now we're tracking the account-level one via CFPrefs.
#define pref_SaveAcctPass		CFSTR("SaveAccountPassword")	// bool: <no equivalent>


#ifdef ARINDAL
# define kDefaultHostAddrInternet		"server.arindal.com:5010"
#else
# define kDefaultHostAddrInternet		"server.deltatao.com:5010"
#endif

enum
{
	kMoveClickHold,
	kMoveClickToggle
};


/*
**	CacheObject
**
**	cached memory objects
**	currently we have images (both with and without custom colors), and sounds
*/
class CacheObject : public DTSLinkedList<CacheObject>
{
public:
	enum CacheObjectType
		{
		kCacheTypeNone,
		kCacheTypeImage,
		kCacheTypeImageColor,
		kCacheTypeSound
		};
	
	const CacheObjectType	coType;		// one of the above flavors
	
	// constructor/destructor
	explicit		CacheObject( CacheObjectType cType ) : coType( cType ) {}
	virtual			~CacheObject();
	
	// no copying
private:
					CacheObject( const CacheObject& );
	CacheObject&	operator=( const CacheObject& rhs );
	
public:
	// overloadable interface
	virtual bool	CanBeDeleted() const;
	
	// interface
	void			Touch();
	bool			IsImage() const
		{
		return kCacheTypeImage == coType || kCacheTypeImageColor == coType;
		}
	
	// for incremental flush
	static bool		RemoveOneObject();
	
	// census data
	static uint		sCachedImageCount;
};

#ifdef USE_OPENGL
class TextureObject;
#endif	// USE_OPENGL


/*
**	ImageCache
**
**	a linked list of cached images
*/
class ImageCache : public CacheObject
{
public:
	CLImage			icImage;
	int				icHeight;
	DTSRect			icBox;
	uint			icUsage;
#ifdef USE_OPENGL
	TextureObject *	textureObject;
	
private:
		// make these private, so they can't be called
		// since we added a pointer member, we need to write
		// explicit constructors if we ever use these
					ImageCache( const ImageCache& );	// copy contructor
	ImageCache&		operator=( const ImageCache& );	// assignment
public:
#endif	// USE_OPENGL
	
	// constructor/destructor
					ImageCache( CacheObjectType cType = kCacheTypeImage ) :
						CacheObject( cType ),
						icUsage( 0 )
#ifdef USE_OPENGL
						, textureObject( nullptr )
#endif
						{
						}
	virtual			~ImageCache();
	
	// interface
	void			DisableFlush();
	void			EnableFlush();
#ifdef IRREGULAR_ANIMATIONS
	void			UpdateAnim( bool bRandomAnims = false );
#endif	// IRREGULAR_ANIMATIONS
	
	// overloaded interface
	virtual bool	CanBeDeleted() const;
};


/*
**	ImageColorCache()
**	like an ImageCache, but with custom mobile colors
*/
class ImageColorCache : public ImageCache
{
public:
	DTSKeyID	icImageID;
	int			icNumColors;
	uchar		icColors[ kNumPlyColors ];

	// constructor/destructor
					ImageColorCache() :
						ImageCache( kCacheTypeImageColor ),
						icImageID( 0 ),
						icNumColors( 0 )
						{}
	virtual			~ImageColorCache();
};


struct DSMobile;

#ifdef AUTO_HIDENAME
enum
{
	// All delays are in frames, using a nominal frame-rate of 4 fps
	// (although the live server generally runs a little faster than that)
	
	kNameDelay_First		= 4 * 10,			// Show name 10 seconds, on first sight
	kNameDelay_Hide			= 1,				// I'd like semi-transparency...
	kNameDelay_MemoryRefresh = 4 * 30 * 30,		// 30 minutes, then redraw the guy
	kNameDelay_Hover		= 2,				// Delay for "player tip"
	kNameDelay_HoverSelf	= 1000,				// longer delay for hovering over self
	kNameDelay_HoverShow	= 8					// Show name for 8 frames after hover
};

// mobile-name fadeout state
enum
{
	kName_Hidden		= 0,
	kName_Visible25,
	kName_Visible50,
	kName_Visible75,
	kName_Visible
};
#endif  // AUTO_HIDENAME


// forward reference to the base element of the player's window list
class PlayerNode;


/*
**	DescTable
**
**	a table of descriptors and their images
*/
const int kMaxDescTextLength		= 512;

// It's necessary to force an alignment here, because descriptors can & do get
// written to disk (in movies), and therefore need to be consistently laid out.
// Forcing 'power' alignment won't cause any harm on 68K machines, but might
// do some good for PPC.
// For future reference: since ~v142, this struct is and has been 668 bytes long.
// The 'descUnused' alignment field was added in v795 to help ensure that.
#pragma pack( push, 4 )

struct DescTable
{
	int32_t				descID;
	int32_t				descCacheID;
	DSMobile *			descMobile;
	int32_t				descSize;
	int32_t				descType;		// v74a: kDescPlayer, kDescMonster, npc, other
	int32_t				descBubbleType;
	int32_t				descBubbleLanguage;
	int32_t				descBubbleCounter;
	int32_t				descBubblePos;
	int32_t				descBubbleLastPos;
	DTSRect				descBubbleBox;
	int32_t				descNumColors;
	DTSPoint			descBubbleLoc;
	uchar				descColors[ kNumPlyColors ];
	char				descName[ kMaxNameLen ];
	DTSRect				descLastDstBox;
	char				descUnused[ 2 ];	// to force 32-bit alignment
	mutable PlayerNode *	descPlayerRef;
#ifdef AUTO_HIDENAME
	int32_t				descSeenFrame;
	int32_t				descNameVisible;		// can be 0, 25, 50 75, 100
#endif
	char				descBubbleText[ kMaxDescTextLength ];
};

#pragma pack( pop )


const int kDescTableBaseSize		= 256;
const int kDescNumberOfThoughts		= 10;
const int kDescTableSize			= kDescTableBaseSize + kDescNumberOfThoughts;


/*
**	DSMobile class
**	a mobile being tracked by the client; sort of a lightweight facade for a DescTable object
*/
struct DSMobile
{
	int		dsmIndex;		// our offset in the gDescTable
	int		dsmState;		// current pose
	int		dsmHorz;		// relative to center of screen
	int		dsmVert;		// ditto
	int		dsmColors;		// various kColorCodeXXX bitflags
};

enum
{
	kBubblePosNone,
	kBubblePosUpperLeft,
	kBubblePosUpperRight,
	kBubblePosLowerRight,
	kBubblePosLowerLeft,
	
	kNumBubblePositions = kBubblePosLowerLeft	// since we start counting at 1
};

enum
{
	kBlitterTransparent = 0,
	kBlitter25,
	kBlitter50,
	kBlitter75
};

enum
{
	move_Stop		= 0,
	move_East,
	move_NorthEast,
	move_North,
	move_NorthWest,
	move_West,
	move_SouthWest,
	move_South,
	move_SouthEast
};
	
enum
{
	move_speed_Walk = 0,
	move_speed_Run,
	move_speed_Stop
};


/*
**	internal inventory commands
*/
enum
{
	kPlyInvCmdNone			= 0,
	kPlyInvCmdEmptyLeft		= -1,
	kPlyInvCmdEmptyRight	= -2
};


/*
**	Global Variables (sigh)
*/
extern DataSpool *	gDSSpool;				// the spool that holds the raw draw state data
extern DescTable *	gDescTable;				// player descriptors
extern DescTable *	gThisPlayer;			// this player's descriptor; MIGHT be stale
						// name of the last character for whom a log was opened, or an empty string
extern char			gLogCharName[ kMaxNameLen ];
extern int			gNumMobiles;			// current number of mobiles
extern DSMobile		gDSMobile[ 256 ];		// mobile table
extern DTSKeyID		gLeftPictID;			// gameWin state data
extern DTSKeyID		gRightPictID;
extern DTSKeyID		gSelectedItemPictID;
extern int			gStateMode;
extern int			gStateMaxSize;
extern int			gStateCurSize;
extern int			gStateExpectSize;
extern uchar *		gStatePtr;
extern PrefsData	gPrefsData;				// preferences
extern bool			gPlayingGame;			// true if we are playing the game
extern bool			gConnectGame;			// true if we are connecting to the game
extern bool			gChecksumMsgGiven;		// we've already complained about a bad checksum,
											// and are about to disconnect
extern bool			gDoneFlag;				// set when the user chooses quit from the file menu
extern bool			gFastBlendMode;			//  your computer is too slow for quality blend mode
extern bool			gInBack;
extern bool			gPlayersListIsStale;
extern int			gFastDrawLimit;
extern int			gSlowDrawLimit;
extern DTSKeyFile	gClientImagesFile;		// the images key file
extern DTSKeyFile	gClientSoundsFile;		// the sounds key file
extern DTSKeyFile	gClientPrefsFile;		// the prefs  key file
extern DTSFileSpec	gMovieFile;				// Movie file being viewed
extern int			gClickFlag;				// where the mouse was clicked
extern bool			gMovePlayer;			// is keyboard movement active?
extern DTSPoint		gClickLoc;				// where the mouse was clicked in the play field
extern int			gClickState;			// non-zero if the mouse is believed to be down
extern DTSWindow *	gGameWindow;			// game window
extern DTSWindow *	gTextWindow;			// text window
extern DTSWindow *	gInvenWindow;			// inventory window
extern DTSWindow *	gPlayersWindow;			// players window
extern int			gAckFrame;				// latest state data frame
extern int			gNumFrames;				// number of frames this session
extern int			gLostFrames;			// number of lost frames this session
extern int			gResendFrame;			// request resend of state data frame
extern ulong		gCommandNumber;			// enumerate the commands sent to the server
extern int			gInvenCmd;				// player's inventory command
extern DTSKeyID		gLeftItemID;			// id of item in the left hand
extern DTSKeyID		gRightItemID;			// id of item in the right hand
extern int			gLastClickGame;			// true if the last click was in the game window
										// Name of the currently selected player or an empty string
extern char			gSelectedPlayerName[ kMaxNameLen ];
extern const char * const sFriendLabelNames[];
extern long			gFrameCounter;			// the frame counter for animations
extern CacheObject * gRootCacheObject;		// cached object linked list
extern int			gCursor;				// ID of the cursor that any idle routine can touch
extern int32_t		gImagesVersion;			// Version numbers of our images...
extern int32_t		gSoundsVersion;			// ... and sounds files
extern const DTSColor	gFriendColors[ kFriendFlagCount ];
#ifdef DEBUG_IMAGECOUNT
extern int			gImageCounts[ kPictFrameIDLimit ];
#endif // DEBUG_IMAGECOUNT
#ifdef MULTILINGUAL
extern int			gRealLangID;			// real language of client
#endif


/*
**	Entry Routines
*/
// Blitters_cl.cp
void	InitBlitters();
void	BlitTransparent( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	FastBlitBlend25( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	FastBlitBlend25Transparent( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	FastBlitBlend50( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	FastBlitBlend50Transparent( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	FastBlitBlend75( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	FastBlitBlend75Transparent( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	QualityBlitBlend25( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	QualityBlitBlend25Transparent( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	QualityBlitBlend50( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	QualityBlitBlend50Transparent( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	QualityBlitBlend75( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );
void	QualityBlitBlend75Transparent( const DTSOffView *, const DTSImage *,
			const DTSRect *, const DTSRect * );

// Cache_cl.cp
void			InitCaches();
ImageCache *	CachePicture( DTSKeyID id );
ImageCache *	CachePicture( DescTable * desc );
ImageCache * 	CachePicture( DTSKeyID id, DTSKeyID * cacheID, int nColors, const uchar * cols );
DTSError		HandleChecksumError( const CLImage * image );
void			ShowChecksumError( DTSKeyID id );
DTSError		ChecksumUsualSuspects();
ImageCache *	CachePictureNoFlush( DTSKeyID id );
void			UpdateAnims();
void			FlushCaches( bool bEverything = false );

// Comm_cl.cp
DTSError	InitReadMovie();
DTSError	InitComm();
void		ExitComm();
void		DoComm();
DTSError	ConnectComm( char challenge[ kChallengeLen ] );
DTSError	SendWaitReply( Message * msg, size_t size, char * error );
char *		AnswerChallenge( char * dst, const char * password, const char * challenge );
void		SetupLogonRequest( LogOn * log, uint messagetag );
void		ResetupLogonRequest( LogOn * log );
char *		GetMagicFileInfo( char * ptr );

// DownloadURL_cl.cp
DTSError	AutoUpdate( LogOn * msg );
DTSError	ExpandFile( DTSFileSpec * file );
void		LaunchNewClient();

// FriendsList_cl.cp
DTSError	SetPermFriend( const char * n, int friendType );
DTSError	ReadFriends( const char * myName );
DTSError	WriteFriends( const char * myName );
int			GetPermFriend( const char * n );
DTSError	InitFriends();

// GameWin_cl.cp
void		InitFrames();
void		InitGameState();
void		CreateCLWindow( const DTSRect * pos );
void		RedrawCLWindow();
void		GetGWMouseLoc( DTSPoint * pt );
void		ExtractImportantData();
void		ShowInfoText( const char * text );
void		ChangeWindowSize();
void		SetBackColors();
void		UpdateCommandsMenu( const char * );
bool	 	MovePlayerToward( int quadrant, int moveSpeed, bool stopIfBalance );
DescTable *	LocateMobileByPoint( const DTSPoint * where, int inKind );
void		TouchHealthBars();
void		KillNotification();
#if DTS_LITTLE_ENDIAN
void		SwapEndian( Layout& );
#endif

// Info_cl.cp
DTSError	CharacterManager();
DTSError	SelectCharacter();
DTSError	EnterPassword();
DTSError	GetHostAddr();

// InventoryWin_cl.cp
void		CreateInvenWindow( const DTSRect * pos );
void		ResetInventory();
void		AddToInventory( DTSKeyID id, int index, bool equipped );
void		InventoryRemoveItem( DTSKeyID id, int index );
void		InventoryEquipItem( DTSKeyID id, bool equip, int index );
void 		InventorySetCustomName( DTSKeyID id, int index, const char * text );
long		InventorySaveSelection();
void		InventoryRestoreSelection( long );
size_t		GetInvItemName( DTSKeyID id, char * dst, size_t maxlen);
void		GetInvItemNameBySlot( int slot, char * dst, size_t maxlen );
void		GetSelectedItemName( char * dst, size_t maxlen );
#ifdef MULTILINGUAL
void		GetInvItemByShortTag( const char * shorttag, char * dst, size_t maxlen );
#endif // MULTILINGUAL
void		ClickSlotObject( int slot );
void		ClearInvCmd();
bool		InventoryShortcut( int ch, uint modifiers );
bool		ResolveItemName( SafeString * name, bool showError );
DTSError	EquipNamedItem( const char * name );
DTSError	SelectNamedItem( const char * name );
DTSError	SendInvCmd( int inCmd, int index = 0 );
void		UpdateInventoryWindow();
DTSError	UpdateItemPrefs();

// Main_cl.cp
void		GetPlayerInput( PlayerInput * pi );
void		SetPlayMenuItems();
void		ShowMsgDlg( const char * text );
void		ShowMsgDlg( CFStringRef text );
void		DrawTextBox( DTSView *, CFStringRef, DTSCoord, DTSCoord, int, ThemeFontID );
DTSCoord	GetTextBoxWidth( const DTSView * view, CFStringRef text, ThemeFontID font );
void		SetBardVolume( int inPct );

#if ENABLE_MUSIC_FILES
// Music_cl.cp
void		RunCLMusic();
void		PlayCLMusic();
bool		CheckMusicCommand( const char * text, size_t len );
#endif  // ENABLE_MUSIC_FILES

// PlayersWin_cl.cp
void		InitPlayers();
void		DisposePlayers();
void		ResetPlayers();
void		CreatePlayersWindow( const DTSRect * pos );
void		RequestPlayersData();
bool		ParseInfoText( const char * text, BEPPrefixID prefix, MsgClasses * );
bool		ParseThinkText( DescTable * desc );
bool		ResolvePlayerName( SafeString * name, bool showError );
void		SetNamedFriend( const char * name, int friendType, bool feedback );
void		SetFriend( int friendType, bool feedback );
void		SetFriendLabel( int newFriends, bool feedback );
void		GiveFriendFeedback( const char * name, int oldlabel, int newlabel );
void		OffsetSelectedPlayer( int delta );
void		SelectPlayer( const char * name, bool toggle = true );
int			GetPlayerIsFriend( const DescTable * desc );
int			GetPlayerIsFriend( const char * name );
int			GetNextFriendLabel( int prevlabel );
int			FriendFlagNumberFromName( const char * name, bool allLabels, bool showError );
void		SetPlayerIsFriend( const DescTable * desc, int friends );
bool		GetPlayerIsDead( const DescTable * desc );
void		SetPlayerIsDead( const DescTable * desc, bool dead );
bool		GetPlayerIsClan( const DescTable * desc );
void		SetPlayerIsClan( const DescTable * desc, bool clan );
bool		GetPlayerIsSharing( const DescTable * desc );
void		SetPlayerIsSharing( const DescTable * desc, bool sharing );
bool		GetPlayerIsSharee( const DescTable * desc );
void		SetPlayerIsSharee( const DescTable * desc, bool sharing );
void		ResetSharees();
DTSError	GatherPlayerFileStats( char * outtext );
void		ListFriends( int label = -1 );
void		ListShares( SafeString * oList, bool bOutbound );

// Sound_cl.cp
void		CLInitSound();
void		CLExitSound();
void		CLPlaySound( DTSKeyID id );
void		SetSoundVolume( uint volume );

// TextPrefs_cl.cp
void		InitTextStyles();
void		GetColorPrefs();
void		SetUpStyle( MsgClasses, CLStyleRecord * );

// TextWin_cl.cp
void		CreateTextWindow( const DTSRect * pos );
void		AppendTextWindow( const char * text );
void		StartTextLog();
void		CloseTextLog();
void		AppendTextWindow( const char * text, const CLStyleRecord * );	// new overload

// Unique_cl.cp
//void		RepelSnerts();

// Update_cl.cp
void		UpdateKeyFiles();
DTSError	UpdateImagesFile( int newversion );
DTSError	UpdateSoundsFile( int newversion );
void		OpenKeyFiles();
void		CloseKeyFiles();

#endif  // CLANLORD_H

