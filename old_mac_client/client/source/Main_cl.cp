/*
**	Main_cl.cp		ClanLord
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

#include <clocale>

// are we using CFPreferences to store app prefs?
#define USE_CF_PREFERENCES		0

// are menus defined in a nib file, or in 'MENU' etc. resources?
#define USE_MBAR_NIB			1

// does the About box use a PNG file, or a QD 'PICT' resource?
#define USE_ABOUT_PNG_FILE		1

// UTI of CL Movies
#define kUTTypeClanLordMovie	CFSTR("com.deltatao.clanlord.movie")


#include "ClanLord.h"
#include "VersionNumber_cl.h"
#include "CommandIDs_cl.h"
#include "Macros_cl.h"
#include "Movie_cl.h"
#include "SendText_cl.h"
#include "Speech_cl.h"
#include "TuneHelper_cl.h"

#if USE_STYLED_TEXT
# include "LaunchURL_cl.h"
#endif	// USE_STYLED_TEXT

#ifdef USE_OPENGL
# include "OpenGL_cl.h"
#endif	// USE_OPENGL

#include "Dialog_mach.h"
#include "GameTickler.h"
#include "KeychainUtils.h"



/*
**	Global Variables
*/
DataSpool *		gDSSpool;				// the spool that holds the raw draw state data
DescTable *		gDescTable;				// player descriptors
DescTable *		gThisPlayer;
int				gNumMobiles;
DSMobile		gDSMobile[ 256 ];

// GameWin state
DTSKeyID		gLeftPictID;
DTSKeyID		gRightPictID;
int				gStateMode;
int				gStateMaxSize;
int				gStateCurSize;
int				gStateExpectSize;
uchar *			gStatePtr;


PrefsData		gPrefsData;
DTSKeyFile		gClientImagesFile;		// the database key file
DTSKeyFile		gClientSoundsFile;		// the database key file
DTSKeyFile		gClientPrefsFile;		// the database key file
DTSFileSpec		gMovieFile;				// Movie file being viewed - BMH
int				gClickFlag;				// where the mouse was clicked
DTSPoint		gClickLoc;				// where the mouse was clicked in the play field
int				gClickState;			// non-zero if the mouse is believed to be down
DTSWindow *		gGameWindow;			// game window
DTSWindow *		gTextWindow;			// text window
DTSWindow *		gInvenWindow;			// inventory window
DTSWindow *		gPlayersWindow;			// players window
int				gAckFrame;				// latest state data frame
int				gNumFrames;				// number of frames this session
int				gLostFrames;			// number of lost frames this session
int				gResendFrame;			// request resend of state data frame
ulong			gCommandNumber = 1;		// enumerate the commands sent to the server
int				gInvenCmd;				// player's inventory command
DTSKeyID		gLeftItemID;			// id of item in the left hand
DTSKeyID		gRightItemID;			// id of item in the right hand
int				gLastClickGame;			// true if the last click was in the game window
										// Name of currently selected player, or an empty string
char 			gSelectedPlayerName[ kMaxNameLen ];
					// Name of the last character for whom a log was opened, or an empty string
char			gLogCharName[kMaxNameLen];
long			gFrameCounter;			// the frame counter for animations
CacheObject *	gRootCacheObject;		// cached object linked list
int				gCursor;				// JEB: ID of the cursor to set to
int32_t			gImagesVersion;			// version numbers of the image...
int32_t			gSoundsVersion;			// ... and sound data files
#if defined( DEBUG_IMAGECOUNT )
int				gImageCounts[ kPictFrameIDLimit ];
#endif
bool			gPlayingGame;			// true if we are playing the game
bool			gConnectGame;			// true if we are connecting to the game
bool			gChecksumMsgGiven;		// true if we've already complained about a bad checksum,
										// and are about to disconnect
bool			gDoneFlag;				// set if the user chooses quit from the file menu
bool			gFastBlendMode;			// set if your computer is too slow for quality blend mode
bool			gMovePlayer;			// is keyboard movement active?
bool			gInBack;
#ifdef MULTILINGUAL
int				gRealLangID = kRealLangDefault;
						// real language of client, set to German by default
#endif


/*
**	Definitions
*/
// use RunApplicationEventLoop()?  We are so not ready for this.
#define RAEL		0

/*
**	class MsgDlg
**
**	Displays a simple, one-string message to the user.
*/
class MsgDlg : public HIDialog, public GameTickler
{
	static const OSType Signature = 'MsgD';
	enum { mdText = 1 };
	
	virtual CFStringRef GetNibFileName() const { return CFSTR("Message"); }
	
	CFStringRef	dlgText;
	
	virtual void		Init();
	
public:
						MsgDlg( CFStringRef msg ) : dlgText( msg )
							{
							if ( dlgText )
								CFRetain( dlgText );
							}
						~MsgDlg()
							{
							if ( dlgText )
								CFRelease( dlgText );
							}
};


#ifdef DEBUG_VERSION
/*
**	class VersionDlg
**
**	allows GMs to fool the server by reporting incorrect version numbers
**	for the sounds, images, and/or app.
*/
class VersionDlg : public HIDialog, public GameTickler
{
	static const OSType Signature = 'SetV';
	static HIViewID	Item( int n ) { return (HIViewID) { Signature, n }; }
	virtual CFStringRef	GetNibFileName() const { return CFSTR("Set Versions"); }
	
	enum
		{
		vdClientEdit = 1,
		vdImagesEdit = 2,
		vdSoundsEdit = 3
		};
	
	int32_t vdClient;
	int32_t vdImages;
	int32_t vdSounds;

	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& );
	
	void	SetVNs() const;
	void	VNtoText( int version, int itemNum ) const;
	void	ResetVNs();
	void	TextToVN( int itemNum, int32_t& version ) const;

public:
	void	SaveInfo();
};
#endif	// DEBUG_VERSION


/*
**	CLMenu class
**
**	Handle selections from the menus.
*/
class CLMenu : public DTSMenu
{
public:
	// overloaded routines
#if USE_MBAR_NIB
	DTSError		Init();	// from nib file
#endif
	virtual bool	DoCmd( long menuCmd );
	
	// interface routines
	void			Setup();
	void			ToggleItem( int32_t * pFlag, int menuCmd );
	void			ToggleMessageLog();
	void			ToggleTextLog();
};


/*
**	CLApp class
**
**	Handle the application in general and key strokes.
*/
class CLApp : public DTSApp, public HICommandResponder, public NavSrvHandler
{
public:
	// overloaded routines
	virtual void	Idle();
	virtual bool	Click( const DTSPoint& loc, ulong time, uint modifiers );
	virtual bool	KeyStroke( int ch, uint modifiers );
	virtual void	Suspend();
	virtual void	Resume();
	
	virtual OSStatus HandleEvent( CarbonEvent& );
	
	virtual OSStatus ProcessCommand( const HICommandExtended&, CarbonEvent& );
	virtual OSStatus UpdateCommandStatus(
						const HICommandExtended& inCmd,
						CarbonEvent& ioEvt,
						EState& outEnable,
						CFStringRef& outTitle ) const;
	
protected:
	// NavSrvHandler stuff
	virtual void		Action( NavCBRec * parm );
	virtual CFArrayRef	CopyFilterTypeIdentifiers() const;
};


/*
**	class MainPrefsDlg
**
**	displays the main preferences
*/
class MainPrefsDlg : public HIDialog, public GameTickler
{
	static const OSType Signature = 'Pref';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	
	virtual CFStringRef		GetNibFileName() const { return CFSTR("Preferences"); }
	virtual CFStringRef		GetPositionPrefKey() const { return CFSTR("MainPrefDlgPosition"); }
	
public:
	int		mcdVolume;
	int		mcdBardVolume;
	
	// overloaded routines
private:
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& );
	
	// interface routines
	void	SaveInfo();
	void	ShowText() const;
	
	// internal
	void	AdjustFriends() const;
	
	enum
		{
		mcdExplanationText			= 1,
		mcdClanningCheck			= 3,
		mcdFallenCheck				= 4,
		mcdNoSuppressFriendsCheck	= 5,
		mcdShowNamesCheck			= 6,
		mcdTimeStampsCheck			= 8,
		mcdBrightColorsCheck		= 9,
		mcdVolumeSlider				= 10,
		mcdBardVolumeSlider			= 11,
		mcdClickRadioGroup			= 12,
		mcdNightPopupMenu			= 13,
		};
	enum
		{
		cmd_AdvancedPrefs			= 'AdvP',
		cmd_ServerAddress			= 'SIpA',
		cmd_ClickRadio				= 'ClkR',
		cmd_SuppressMsgs			= '!Msg',
		cmd_SetVolume				= 'SVol',
		cmd_SetBardVolume			= 'SBVl',
		};
};


/*
**	class AboutBox
**
**	runs our funky almost-minimal about-box dialog
**	(with help from the Spinny class for the animated DTS logo)
*/
class Spinny;
class AboutBox : public HIDialog, public GameTickler
{
	static const OSType Signature = 'Abou';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	
	virtual CFStringRef GetNibFileName() const { return CFSTR("About"); }
	
	enum {
		abBlurb = 1,
		abSpinner = 2,
		abPicture,
		};
	
	virtual void	Init();
# if USE_ABOUT_PNG_FILE
	void			SetAboutPicture() const;
# endif
	
	Spinny *	abSpinny;

public:
				AboutBox() : abSpinny() {}
				~AboutBox();
};


/*
**	class NetStatsDlg
**
**	displays packet-level statistics for session & lifetime
*/
class NetStatsDlg : public HIDialog, public GameTickler
{
	typedef HIDialog SUPER;
	static const OSType Signature = 'NetI';
	static HIViewID Item( int n ) { return (HIViewID) { Signature, n }; }
	
	virtual CFStringRef GetNibFileName() const { return CFSTR("Network Stats"); }
	virtual CFStringRef	GetPositionPrefKey() const { return CFSTR("NetStatsDlgPosition"); }
	
	CFNumberFormatterRef nsdFormatter;
	int		nsdNumFrames;
	
	// overloaded routines
	virtual void	Init();
	virtual void	Idle();
	
	void			Update();
	void			SetNumber( const HIViewID& item, int value );
	
	enum
		{
		nsdSessionNumFrames		= 1,
		nsdSessionLostFrames	= 2,
		nsdSessionPercentLost	= 3,
		nsdTotalNumFrames		= 4,
		nsdTotalLostFrames		= 5,
		nsdTotalPercentLost		= 6,
		
		nsdSampleLatency		= 7,
		nsdMeanLatency			= 8,
		
		nsdOk					= 9,
		};
	
public:
					NetStatsDlg() : nsdFormatter( nullptr ), nsdNumFrames( 0 ) {}
					~NetStatsDlg()
						{
						if ( nsdFormatter )
							CFRelease( nsdFormatter );
						}
};


/*
**	class LatencyRecorder
**	cheap & cheezy estimator of roundtrip packet latency between you and the CL Server
*/
class LatencyRecorder
{
	// what we sample
	struct CmdAckRecord
	{
		CFAbsoluteTime	carTimestamp;
		uint			carNum;
		uint			carLatency;
	};
	
	static const int kNumSavedAcks = 10;
	
	CmdAckRecord	ackTimes[ kNumSavedAcks ];		// circular buffer of saved samples
	int				mAckIndex;						// buffer index
	uint			mTotalLatency;					// running total
	uint			mNumLatencySamples;				// running average
	
	void		Calc1Latency( int sampleIndex, CFAbsoluteTime when );
	
public:
				LatencyRecorder();
				~LatencyRecorder() {}
	
	void		RecordSentCmd( ulong cmdNum );
	void		RecordRecvCmd( ulong cmdNum );
	
	void		GetStats( float& sampleLatency, float& meanLatency ) const;
	
private:	// no copying
				LatencyRecorder( const LatencyRecorder& );
	LatencyRecorder& operator=( const LatencyRecorder& );
};


// this ought to be in DataBaseTypes.h...
const DTSKeyType kTypePreferences	= 'pref';


// Apple- and CarbonEvent-related codes
// TODO: move these to a .h file so the .r file can see 'em...
enum
{
	kRecordMovieEvent		= 'Rec ',
	kEquipItemEvent			= 'Eqip',		// only if TEST_EQUIP_ITEM
	kJoinGameEvent			= 'AuDL',
	kConnectionStatusEvent	= 'IsCn',
	
	kEvent_ViewMovie		= 'VMuv'		// private internal CarbonEvent for viewing a movie
};


/*
**	Internal Routines
*/
static void		CheckHostRequirements();
static void		GetPrefsData();
#if DTS_LITTLE_ENDIAN && ! USE_CF_PREFERENCES
static void		PrefsSwapEndian();
#endif  // DTS_LITTLE_ENDIAN
static void		SavePrefsData();
static void		DoCharacterManager();
static void		DoSelectMovieFile();
static OSStatus	DidChooseMovieFile( CFURLRef url );
static void		DoSelect();
static void		DoJoin();
static void		PlayGame();
static void		SelectShowWindow( DTSWindow * window );
static bool		DoWindowCmd( long menuCmd );
static void		AllocDescTable();
static void		GetWindowPosition( const DTSWindow * window, DTSRect * pos );
static void		GetMainPrefs();
static void		ValidateWindowPosition( DTSRect * pos, int windowNumber );
static void		InitScreens();
static void		ChooseDefaultWindowPositions();
#ifdef DEBUG_VERSION
static void		GetDrawRate();
#endif  // DEBUG_VERSION
static void		OSOpen();
static void		OSNew();
static void		DoRecordMovie();
static void		SetAdvancedPrefs();
static void		SetCommandQStatus( bool enabled );
static DTSError	InstallMovieHandler();
static DTSError	InstallAutoDownloadHandler();
static SInt32	MyGetFileID( const char * filename );
static void		MovePWsToKeychain();
static void		ShowNetworkStats();
static void		BadgeDockIcon();
static void		ResetDockIcon();

#if defined( DEBUG_IMAGECOUNT )
static void		DumpImageCounts();
#endif


/*
**	Variables
*/
enum WindowNumbers
{
	kGameWindow = 0,
	kTextWindow,
	kInvenWindow,
	kPlayersWindow,
	
	kNumWindows
};


static DTSRect		gDefaultWindowPositions[ kNumWindows ];
static int			gNumScreens;
static DTSRect *	gScreenBounds;
static bool			gDoComm;

// QT Music files
#if ENABLE_MUSIC_FILES
static bool			gMusicDisabled;
#endif

// the one & only recorder
static LatencyRecorder		gLatencyRecorder;

#ifdef HANDLE_MUSICOMMANDS
extern CTuneQueue	gTuneQueue;
#endif


/*
**	main()
**
**	Initialize everything.
**	Run the application.
**	Look for messages from the server.
*/
int
main()
{
	setlocale( LC_ALL, "" );
	
	CLApp theCLApp;
	CLMenu theMenu;
	
	// make sure we can run
	CheckHostRequirements();
	
	// Enable debugging messages
#ifdef DEBUG_VERSION
	bDebugMessages = true;
#endif	// DEBUG_VERSION
	
	// set WNE time -- this number gets reduced when we actually start playing
	theCLApp.SetSleepTime( ::GetCaretTime() );
	
	gErrorCode = noErr;
	
	// enumerate the screens
	InitScreens();
	
	// Initalize the default window position rects
	ChooseDefaultWindowPositions();
	
#if defined( DEBUG_IMAGECOUNT )
	// initialize the image stats array
	memset( gImageCounts, 0, sizeof gImageCounts );
#endif // DEBUG_IMAGECOUNT
	
	// Initialize text styles
	InitTextStyles();
	
	// open the key files
	if ( not gErrorCode )
		OpenKeyFiles();
	
	// get the prefs data
	// regardless of whether there was an error opening the key files or not
	GetPrefsData();
	
	// Update the files
	if ( not gErrorCode )
		UpdateKeyFiles();
	
	if ( not gErrorCode )
		{
		// Make sure the window positions make sense.
		ValidateWindowPosition( &gPrefsData.pdGWPos,      kGameWindow    );
		ValidateWindowPosition( &gPrefsData.pdTextPos,    kTextWindow    );
		ValidateWindowPosition( &gPrefsData.pdInvenPos,   kInvenWindow   );
		ValidateWindowPosition( &gPrefsData.pdPlayersPos, kPlayersWindow );
		
		// Under the Aqua appearance, windows can't be dragged by their
		// edges anymore. So we have to ensure that the titlebars of each
		// window is on a screen as well.
		const DTSCoord kTitleBarHeight = 22;	// close enough
		
		DTSRect titleBar;
		titleBar = gPrefsData.pdGWPos;
		titleBar.rectBottom = titleBar.rectTop + kTitleBarHeight;
			// the exact height of the bar is not important
		ValidateWindowPosition( &titleBar, kGameWindow );
		
		titleBar = gPrefsData.pdTextPos;
		titleBar.rectBottom = titleBar.rectTop + kTitleBarHeight;
		ValidateWindowPosition( &titleBar, kTextWindow );
		
		titleBar = gPrefsData.pdInvenPos;
		titleBar.rectBottom = titleBar.rectTop + kTitleBarHeight;
		ValidateWindowPosition( &titleBar, kInvenWindow );
		
		titleBar = gPrefsData.pdPlayersPos;
		titleBar.rectBottom = titleBar.rectTop + kTitleBarHeight;
		ValidateWindowPosition( &titleBar, kPlayersWindow );
		}
	
	// initialize the blitters
	if ( not gErrorCode )
		InitBlitters();
	
#if ENABLE_MUSIC_FILES
	// initialize music engine (QuickTime background music, not bards)
	if ( not gErrorCode )
		{
		DTSError result = InitMusic();
		if ( result != noErr )
			{
			gMusicDisabled = true;
			gErrorCode = noErr;
			}
		}
#endif  // ENABLE_MUSIC_FILES
	
	// initialize the menus
#if USE_MBAR_NIB
	if ( not gErrorCode )
		gErrorCode = theMenu.Init();
#else
	if ( not gErrorCode )
		gErrorCode = theMenu.Init( rMenuBarID );
	if ( not gErrorCode )
		theMenu.AddMenu( rLabelSubmenuID, kInsertHierarchicalMenu );
#endif  // USE_MBAR_NIB
	
	if ( not gErrorCode )
		theMenu.Setup();
	
	// allocate the descriptor table
	if ( not gErrorCode )
		AllocDescTable();
	
	// initialize the caches
	if ( not gErrorCode )
		InitCaches();
	
	// initialize the frame buffer
	if ( not gErrorCode )
		InitFrames();
	
	//JEB Init the LaunchURLHandler
	if ( not gErrorCode )
		gErrorCode = gLaunchURLHandler.Init();
	
#if USE_SPEECH
	// init the speech engine
	if ( not gErrorCode )
		InitSpeech();
#endif
	
	if ( not gErrorCode )
		{
#if DEBUG_VERSION
//		ShowMessage( _(TXTCL_WIN_TITLE), kFullVersionString );
#endif	// DEBUG_VERSION
		
		// create the players window
		CreatePlayersWindow( &gPrefsData.pdPlayersPos );
		
		// create the inventory window
		CreateInvenWindow( &gPrefsData.pdInvenPos );
		
		// create the text window
		CreateTextWindow( &gPrefsData.pdTextPos );
		
		// create the game window
		CreateCLWindow( &gPrefsData.pdGWPos );
		
#ifdef USE_OPENGL
		gOpenGLEnableEffects = gPrefsData.pdOpenGLEnableEffects;
		switch ( gPrefsData.pdOpenGLRenderer )
			{
			case 2:
				gOpenGLForceGenericRenderer = false;
				gOpenGLPermitAnyRenderer = true;
				break;
			
			case 1:
				gOpenGLForceGenericRenderer = true;
				gOpenGLPermitAnyRenderer = false;
				break;
			
			case 0:
			default:
				gOpenGLForceGenericRenderer = false;
				gOpenGLPermitAnyRenderer = false;
				break;
			}
		
		if ( isOpenGLAvailable() )	// also sets gOpenGLAvailable
			{
			gOpenGLEnable = gPrefsData.pdOpenGLEnable;
			if ( gOpenGLEnable )
				{
				attachOGLContextToGameWindow();
				if ( ctx )
					{
					initOpenGLGameWindowFonts( ctx );
					setupOpenGLGameWindow( ctx, gPlayingGame );
					gOpenGLEnable = true;
					gPrefsData.pdOpenGLEnable = 1;
					}
				else
					{
					gOpenGLEnable = false;
					gPrefsData.pdOpenGLEnable = 0;
					}
				}
			}
		else
			gOpenGLEnable = false;
#endif	// USE_OPENGL
		}
	
	// install app scroll wheel handler
	if ( not gErrorCode )
		{
		gErrorCode = theCLApp.InitTarget( GetApplicationEventTarget() );
		if ( noErr == gErrorCode )
			{
			const EventTypeSpec evt = { kClientAppSign, kEvent_ViewMovie };
			gErrorCode = theCLApp.AddEventType( evt );
			}
		}
	
#if ! USE_MACRO_FOLDER
	// initialize macros
	if ( not gErrorCode )
		InitMacros();
#endif
	
	// initialize permanent friends
	InitFriends();
	
	// set up for movie applescript command
	if ( not gErrorCode )
		InstallMovieHandler();
	
	SetCommandQStatus( true );		// cmd-q is enabled until we are connected
	
	InstallAutoDownloadHandler();
	
	// play the game until the user quits
	// (This is the "outer" main event loop; there's also an "inner main loop" at PlayGame().
	// Somehow, these two should be merged.)
	if ( not gErrorCode )
		{
		gDoneFlag = false;
		
#if RAEL
		RunApplicationEventLoop();
#else
		while ( not gDoneFlag )
			{
#if ENABLE_MUSIC_FILES
			if ( not gMusicDisabled )
				RunCLMusic();
#endif
			RunApp();
			}
#endif  // RAEL
		}
	
#if defined( DEBUG_IMAGECOUNT )
	DumpImageCounts();
#endif
	
#if USE_SPEECH
	// shut down speech engine
	ExitSpeech();
#endif
	
#if ENABLE_MUSIC_FILES
	// shut down music engine
	if ( not gMusicDisabled )
		ExitMusic();
#endif
	
	// save the window positions
	SavePrefsData();
	
	// close the movie file
	CCLMovie::StopMovie();
	
	// close the message window
	CloseMsgWin();
	
	// close the Text log file
	CloseTextLog();
	
	// close the key files
	CloseKeyFiles();
	
	// Unbadge the icon
	ResetDockIcon();
	
#if defined( USE_OPENGL )
	// we're about to toss function pointers, so we want to be as
	// "done" as we possibly can be so nothing can call the draw/redraw/etc
	
	// QUERY: is this "the" exit?  no other exit() or exittoshell() or atexit()
	// hidden anywhere?  because if there are, we need to track them all down
	// and make sure they all call this as well.
	// ... doesn't matter any more, since aglDeallocEntryPoints() is now a noop
	
	aglDeallocEntryPoints();	// cleanup and restore gBundleRefOpenGL to NULL
#endif	// USE_OPENGL
	
	// if we downloaded a new client, launch it now.
	LaunchNewClient();
	
	return EXIT_SUCCESS;
}


/*
**	CheckHostRequirements()
**
**	verify that we have the necessary OS versions etc.
*/
void
CheckHostRequirements()
{
	int reason = 0;		// assume we're OK

	// need OS 10.6+ (well, almost...)
	enum {
#if MAC_OS_X_VERSION_MIN_REQUIRED <= 1050
		kMinOSLevel = 5
#else
		kMinOSLevel = 6
#endif
	};

	if ( gOSFeatures.osVersionMajor  < 10 ||
	    (10 == gOSFeatures.osVersionMajor && gOSFeatures.osVersionMinor < kMinOSLevel) )
		reason = 1;
	
	// need QuickTime
	if ( not gOSFeatures.hasQuickTime )
		reason = 2;
	
	// are we unsatisfied?
	if ( reason )
		{
		// TRANSLATE PLEASE
		GenericError( "Sorry, this computer does not meet the minimum requirements "
			"to play " TXTCL_APP_NAME "." );
		
		exit( EXIT_SUCCESS );
		}
}


/*
**	InitScreens()
**
**	Count the screens and get their sizes.
*/
void
InitScreens()
{
	gNumScreens = GetNumberOfScreens();
	gScreenBounds = NEW_TAG("ScreenRects") DTSRect[ gNumScreens ];
	
	for ( int cnt = 0; cnt < gNumScreens; ++cnt )
		GetScreenBounds( cnt, &gScreenBounds[ cnt ] );
}


/*
**	ChooseDefaultWindowPositions()
**
**	set up the raw default window positions
**	these are only used if the stored positions (in the prefs)
**	are missing or invalid.
*/
void
ChooseDefaultWindowPositions()
{
	DTSRect screen;
	GetMainScreen( &screen );
	
	// leave room for window title bars
	screen.rectTop += 22;
	
	// leave a bit of a border (extra if "puffy", but don't force window title bars down too far)
	screen.Inset( 10, 10 );
	screen.rectTop -= 4;
	
	enum
		{
		// these are just rough guides; each window will fine-tune itself as it opens
		kGameWidth		= 640,
		kGameHeight		= 480,
		
		kTextWidth		= 512,
		kTextHeight		= 20 * 16,	// 20 lines @ 16 pixels
		
		kInvenWidth		= 215,
		kInvenHeight	= 12 * 17,	// 12 items is plenty for new folks
		
		kPlayerWidth	= 215,
		kPlayerHeight	= 20 * 18	// 20 players
		};
	
	// attempt to tile the windows, sort of:
	// main window in upper left
	gDefaultWindowPositions[ kGameWindow    ].Set(
			screen.rectLeft,
			screen.rectTop,
			screen.rectLeft + kGameWidth,
			screen.rectTop + kGameHeight );
	
	// text window at lower left
	gDefaultWindowPositions[ kTextWindow    ].Set(
			screen.rectLeft,
			screen.rectBottom - kTextHeight,
			screen.rectLeft + kTextWidth,
			screen.rectBottom );
	
	// player window at upper right
	gDefaultWindowPositions[ kPlayersWindow ].Set(
			screen.rectRight - kPlayerWidth,
			screen.rectTop,
			screen.rectRight,
			screen.rectTop + kPlayerHeight );
	
	// inven window at lower right
	gDefaultWindowPositions[ kInvenWindow   ].Set(
			screen.rectRight - kInvenWidth,
			screen.rectBottom - kInvenHeight,
			screen.rectRight,
			screen.rectBottom );
}


/*
**	ValidateWindowPosition()
**
**	Make sure that the window is in a valid position.
*/
void
ValidateWindowPosition( DTSRect * pos, int windowNumber )
{
	// check each active screen
	for ( int cnt = 0; cnt < gNumScreens; ++cnt )
		{
		DTSRect screenBounds = gScreenBounds[ cnt ];
		screenBounds.Inset( 10, 10 );
		
		// if 'pos' has a non-empty intersection with that screen,
		// i.e. if the two rects overlap, then this window's position is acceptable
		screenBounds.Intersect( pos );
		if ( not screenBounds.IsEmpty() )
			return;
		}
	
	// must be bad -- no part is visible on any screen
	*pos = gDefaultWindowPositions[ windowNumber ];
	
#if 0  // small window option no longer supported
	if ( kGameWindow == windowNumber )
		gPrefsData.pdLargeWindow = false;
#endif  // 0
}


/*
**	MsgDlg::Init()
*/
void
MsgDlg::Init()
{
	SetText( (HIViewID) { Signature, mdText }, dlgText );
}


/*
**	ShowMsgDlg()
**
**	pop up a dialog that shows some text
*/
void
ShowMsgDlg( const char * text )
{
	CFStringRef cs = CreateCFString( text );
	__Check( cs );
	if ( cs )
		{
		MsgDlg dlg( cs );
		dlg.Run();
		CFRelease( cs );
		}
}


/*
**	ShowMsgDlg()
**
**	pop up a dialog that shows some text (CFString flavor)
*/
void
ShowMsgDlg( CFStringRef cs )
{
	MsgDlg dlg( cs );
	dlg.Run();
}


#if defined( DEBUG_IMAGECOUNT )
/*
**	DumpImageCounts()
**
**	dump the image-count data to text file
*/
void
DumpImageCounts()
{
	ShowMessage( "Saving image counts..." );
	
	// save the image count data
	DTSFileSpec spec;
	spec.GetCurDir();
	spec.SetFileName( "ImageCounts.txt" );
	
	// open a stream on the file, write out the counts
	if ( FILE * stream = spec.fopen( "w" ) )
		{
		for ( uint i = 0 ; i < kPictFrameIDLimit ; ++i )
			fprintf( stream, "%u\t%d\n", i, gImageCounts[ i ] );
		
		fclose( stream );
		}
}
#endif  // DEBUG_IMAGECOUNT


/*
**	SendPlyrCommand()
**
**	shorthand for a quiet SendCommand(), using the currently selected player's name
*/
static bool
SendPlyrCommand( const char * cmd )
{
	return SendCommand( cmd, gSelectedPlayerName, true );
}


/*
**	CLMenu::DoCmd()
**
**	Handle selections from the menu.
*/
bool
CLMenu::DoCmd( long menuCmd )
{
	bool result = true;
	switch ( menuCmd )
		{
		
// AppleEvents
		case mOSOpen:
			OSOpen();
			break;
		
		case mOSNew:
			OSNew();
			break;
		
// Apple menu
		case mAbout:
			{
			AboutBox dlg;
			dlg.Run();
			}
			break;
		
// File menu
		case mCharManager:
			DoCharacterManager();
			break;
		
		case mSelectChar:
			DoSelect();
			break;
		
		case mJoinGame:
			DoJoin();
			break;
		
		case mDisconnect:
			if ( CCLMovie::IsRecording() )
				CCLMovie::StopMovie();
			gPlayingGame = false;
			break;
		
		case mViewMovie:
			DoSelectMovieFile();
			break;
		
		case mRecordMovie:   // BMH timmer
			DoRecordMovie();
			break;
		
		case mOSQuit:	// not in File menu anymore; this command now comes in via
						// a kAEQuitApplication apple-event
			gDoneFlag = true;
#if RAEL
			QuitApplicationEventLoop();
#endif
			break;
		
// Edit Menu
		case mUndo:
		case mCut:
		case mCopy:
		case mPaste:
		case mClear:
		case mSelectAll:
		case mFind:
		case mFindAgain:
			result = DoWindowCmd( menuCmd );
			break;
		
// Options menu
		case mSound:
			ToggleItem( &gPrefsData.pdSound, menuCmd );
#ifdef HANDLE_MUSICOMMANDS
			if ( not gPrefsData.pdSound )
				gTuneQueue.StopPlaying();		// stop any bard music when turn sounds off
#endif	// HANDLE_MUSICOMMANDS
			break;
		
#if ENABLE_MUSIC_FILES
		case mMusicPlay:
			if ( not gMusicDisabled )
				{
				PlayCLMusic();
				uint flags = kMenuNormal;
				if ( gPrefsData.pdMusicPlay )
					flags = kMenuChecked;
				SetFlags( mMusicPlay, flags );
				}
			break;
#endif  // ENABLE_MUSIC_FILES
		
		case mTextColors:
			GetColorPrefs();
			break;
		
		case mReloadMacros:
			InitMacros();
			break;
		
		case mSaveTextLog:
			ToggleTextLog();
			break;
		
		case mNetworkStats:
			ShowNetworkStats();
			break;
		
		case mGameWindow:
			SelectShowWindow( gGameWindow );
			break;
		
		case mTextWindow:
			SelectShowWindow( gTextWindow );
			break;
		
		case mInvenWindow:
			SelectShowWindow( gInvenWindow );
			break;
		
		case mPlayersWindow:
			SelectShowWindow( gPlayersWindow );
			break;
		
// Command menu
		case mInfo:
			SendPlyrCommand( "info" );
			break;
		
		case mPull:
			SendPlyrCommand( "pull" );
			break;
		
		case mPush:
			SendPlyrCommand( "push" );
			break;
		
		case mKarma:
			SendPlyrCommand( "karma" );
			break;
		
		case mThank:
			SendPlyrCommand( "thank" );
			break;
		
		case mCurse:
			SendPlyrCommand( "curse" );
			break;
		
		case mShare:
			SendPlyrCommand( "share" );
			break;
		
		case mUnshare:
			SendPlyrCommand( "unshare" );
			break;
		
		case mBefriend:
//			SetFriendLabel1( true );	// replaced by label submenu
			break;
		
		case mBlock:
			SetFriend( kFriendBlock, true );
			break;
		
		case mIgnore:
			SetFriend( kFriendIgnore, true );
			break;
		
		case mEquipItem:
		case mUseItem:
			result = gInvenWindow->DoCmd( menuCmd );
			break;
		
// labels submenu
		case mNoLabel:
			SetFriendLabel( kFriendNone, true );
			break;
		
		case mRedLabel:
		case mOrangeLabel:
		case mGreenLabel:
		case mBlueLabel:
		case mPurpleLabel:
				// rely on cmd-code and label-code ranges both being contiguous
			SetFriendLabel( kFriendLabel1 + menuCmd - mRedLabel, true );
			break;
		
#ifdef DEBUG_VERSION
// God menu
		case mShowMonsterNames:
			ToggleItem( &gPrefsData.pdGodHideMonsterNames, menuCmd );
			break;

		case mAccount:
			SendPlyrCommand( "account" );
			break;
		
		case mSet:
			SendPlyrCommand( "set" );
			break;
		
		case mStats:
			SendPlyrCommand( "stats" );
			break;
		
		case mRanks:
			SendPlyrCommand( "ranks" );
			break;
		
		case mGoto:
			SendPlyrCommand( "goto" );
			break;
		
		case mWhere:
			SendPlyrCommand( "where" );
			break;
		
		case mPunt:
			SendPlyrCommand( "punt" );
			break;
		
		case mTricorder:
			{
			char txt[256];
			snprintf( txt, sizeof txt, "useitem swissarmy \"%s\" ;tric", gSelectedPlayerName );
			SendCommand( txt, nullptr, true );
			} break;
		
		case mFollow:
			SendPlyrCommand( "follow" );
			break;
		
// Debug menu
		case mShowFrameTime:
			ToggleItem( &gPrefsData.pdShowFrameTime, menuCmd );
			break;
		
		case mSaveMessageLog:
			ToggleMessageLog();
			break;
		
		case mShowDrawTime:
			ToggleItem( &gPrefsData.pdShowDrawTime, menuCmd );
			break;
		
		case mShowImageCount:
			ToggleItem( &gPrefsData.pdShowImageCount, menuCmd );
			break;
		
		case mFlushCaches:
			FlushCaches();
			break;
		
		case mSetIPAddress:
			GetHostAddr();
			break;
		
		case mFastBlitters:
			ToggleItem( &gPrefsData.pdFastBlitters, menuCmd );
			break;
		
		case mDrawRateControls:
			GetDrawRate();
			break;
		
		case mDumpBlocks:
			break;
		
		case mPlayerFileStats:
			{
			char buff[ 256 ];
			if ( not GatherPlayerFileStats( buff ) )
				{
				if ( CFStringRef cs = CreateCFString( buff ) )
					{
					ShowMsgDlg( cs );
					CFRelease( cs );
					}
				}
			else
				Beep();
			} break;
		
		case mSetVersionNumbers:
			{
			VersionDlg versdlg;
			versdlg.Run();
			
			if ( kHICommandOK == versdlg.Dismissal() )
				versdlg.SaveInfo();
			} break;
#endif	// DEBUG_VERSION
	
// Help menu
		case mClientHelpURL:
			result = gURLMenu.DoCmd( menuCmd );
			break;
		
// all others
		default:
			result = DTSMenu::DoCmd( menuCmd );
			break;
		}
	
	return result;
}


#if USE_MBAR_NIB
/*
**	CLMenu::Init()
**
**	override superclass to initialize the menu bar from a nib-file, not resources
*/
DTSError
CLMenu::Init()
{
	// paranoia
	if ( not this->priv.p )
		return -1;
	
	// install main menu bar
	MenuRef tmp = nullptr;
	IBNibRef nib = nullptr;
	OSStatus err = CreateNibReference( CFSTR("MainMenu"), &nib );
	__Check_noErr( err );
	if ( noErr == err )
		{
		err = SetMenuBarFromNib( nib, CFSTR("MainMenu") );
		__Check_noErr( err );
		}
	if ( nib )
		DisposeNibReference( nib );
	
# ifdef DEBUG_VERSION
	// install debug menus
	nib = nullptr;
	if ( noErr == err )
		{
		err = CreateNibReference( CFSTR("DebugMenus"), &nib );
		__Check_noErr( err );
		
		if ( noErr == err )
			{
			tmp = nullptr;
			err = CreateMenuFromNib( nib, CFSTR("God"), &tmp );
			__Check_noErr( err );
			if ( noErr == err && tmp )
				InsertMenu( tmp, 0 );	// at end
			}
		if ( noErr == err )
			{
			tmp = nullptr;
			err = CreateMenuFromNib( nib, CFSTR("Debug"), &tmp );
			__Check_noErr( err );
			if ( noErr == err && tmp )
				InsertMenu( tmp, 0 );
			}
		if ( noErr == err && bDebugMessages )
			{
			tmp = nullptr;
			err = CreateMenuFromNib( nib, CFSTR("Prerelease"), &tmp );
			__Check_noErr( err );
			if ( noErr == err && tmp )
				InsertMenu( tmp, 0 );
			}
		}
	if ( nib )
		DisposeNibReference( nib );
# endif  // DEBUG_VERSION
	
	// install the standard Window menu
	if ( noErr == err )
		{
		tmp = nullptr;
		err = CreateStandardWindowMenu( kWindowMenuIncludeRotate, &tmp );
		__Check_noErr( err );
		if ( noErr == err )
			InsertMenu( tmp, 0 );
		}
	
	return err;
}
#endif  // USE_MBAR_NIB


/*
**	CLMenu::Setup()
**
**	init the menu items
*/
void
CLMenu::Setup()
{
#if ! USE_MBAR_NIB
	// Initially, the Labels submenu is only somewhat loosely attached to its parent Commands
	// menu (by having its menuID stored in the latter's hMenuChecked field, in the relevant
	// MENU resource in ClanLord.r).  Until we switch to a nib-based menu bar, it's necessary
	// to strengthen that attachment, as follows.
	if ( true )
		{
		MenuRef labelSubMenu = GetMenuHandle( rLabelSubmenuID );
//		__Check( labelSubMenu );
		MenuRef cmdMenu = GetMenuHandle( rCommandMenuID );
//		__Check( cmdMenu );
		if ( cmdMenu && labelSubMenu )
			__Verify_noErr( SetMenuItemHierarchicalMenu( cmdMenu,
								LoWord(mBefriend), labelSubMenu ) );
		}
#endif  // ! USE_MBAR_NIB
	
	gPrefsData.pdGodHideMonsterNames = not gPrefsData.pdGodHideMonsterNames;
	DoCmd( mShowMonsterNames );
	
	// set the Save Text Log menu checkmark, but don't actually start a new log yet
	gPrefsData.pdSaveTextLog = not gPrefsData.pdSaveTextLog;
	ToggleItem( &gPrefsData.pdSaveTextLog, mSaveTextLog );
//	DoCmd( mSaveTextLog );
	
#ifdef DEBUG_VERSION
	gPrefsData.pdShowFrameTime = not gPrefsData.pdShowFrameTime;
	DoCmd( mShowFrameTime );
	
	gPrefsData.pdSaveMsgLog = not gPrefsData.pdSaveMsgLog;
	DoCmd( mSaveMessageLog );
	
	gPrefsData.pdShowDrawTime = not gPrefsData.pdShowDrawTime;
	DoCmd( mShowDrawTime );
	
	gPrefsData.pdShowImageCount = not gPrefsData.pdShowImageCount;
	DoCmd( mShowImageCount );
	
	gPrefsData.pdFastBlitters = not gPrefsData.pdFastBlitters;
	DoCmd( mFastBlitters );
#endif	// DEBUG_VERSION
	
	gPrefsData.pdSound = not gPrefsData.pdSound;
	DoCmd( mSound );
	
#if ENABLE_MUSIC_FILES
	if ( not gMusicDisabled )
		{
		gPrefsData.pdMusicPlay = not gPrefsData.pdMusicPlay;
		DoCmd( mMusicPlay );
		}
	else
#endif
		SetFlags( mMusicPlay, kMenuDisabled );
	
	SetPlayMenuItems();
	
	// Initialize the URLmenu
	// * get this text from a resource eventually...
	SetText( mClientHelpURL, _(TXTCL_CLIENT_WEB_HELP) );
	
	// set its commandID
	if ( 1 )
		{
		MenuRef helpMenuRef = nullptr;
		MenuItemIndex firstItem = 0;
		OSStatus err = HMGetHelpMenu( &helpMenuRef, &firstItem );
		__Check_noErr( err );
		if ( noErr == err )
			{
			__Verify_noErr( SetMenuItemCommandID( helpMenuRef, firstItem, cmd_ClientHelpURL ) );
			}
		}

	// Enable "Preferences" command in app menu
	EnableMenuCommand( nullptr, kHICommandPreferences );
}


/*
**	CLMenu::ToggleItem()
**
**	toggle the boolean and check the menu item
*/
void
CLMenu::ToggleItem( int32_t * pFlag, int menuCmd )
{
	bool flag = not *pFlag;
	*pFlag = flag;
	uint menuFlags = flag ? kMenuChecked : kMenuNormal;
	SetFlags( menuCmd, menuFlags );
}


/*
**	CLMenu::ToggleMessageLog()
**
**	toggle the message log
*/
void
CLMenu::ToggleMessageLog()
{
	ToggleItem( &gPrefsData.pdSaveMsgLog, mSaveMessageLog );
	if ( gPrefsData.pdSaveMsgLog )
		SaveMsgLog( "CLMsgLog.txt" );
	else
		SaveMsgLog( nullptr );
}


/*
**	CLMenu::ToggleTextLog()
**
**	toggle the text log
*/
void
CLMenu::ToggleTextLog()
{
	ToggleItem( &gPrefsData.pdSaveTextLog, mSaveTextLog );
	StartTextLog();
}


/*
**	PerformAutoJoin()
**
**	queue up a Join command request
*/
static OSStatus
PerformAutoJoin()
{
	HICommandExtended cmd;
	cmd.attributes = kHICommandFromMenu;
	cmd.commandID = cmd_JoinGame; /*'Join';*/	// not yet used, but maybe someday...
	cmd.source.menu.menuRef = GetMenuHandle( rFileMenuID );
	cmd.source.menu.menuItemIndex = LoWord( mJoinGame );
	
	CarbonEvent menuEvt( kEventClassCommand, kEventCommandProcess );
	OSStatus err;
//	if ( noErr == err )
		{
		err = menuEvt.SetParameter( keyDirectObject, typeHICommand, sizeof cmd, &cmd );
		__Check_noErr( err );
		}
	if ( noErr == err )
		{
		err = menuEvt.Post( nullptr, GetMainEventQueue() );
		__Check_noErr( err );
		}
	
	return err;
}


/*
**	OSOpen()
*/
void
OSOpen()
{
	bool bIsMovieFile = false;
	
	// check the file type (UTI)
	CFTypeRef tr = nullptr;
	
#if MAC_OS_X_VERSION_MIN_REQUIRED > 1050
	// we can use the new URL-based accessor
	if ( CFURLRef url = DTSFileSpec_CreateURL( &gDTSOpenFile ) )
		{
		CFURLCopyResourcePropertyForKey( url, kCFURLTypeIdentifierKey, &tr, nullptr );
		CFRelease( url );
		}
#else
	FSRef fsr;
	if ( noErr == DTSFileSpec_CopyToFSRef( &gDTSOpenFile, &fsr ) )
		LSCopyItemAttribute( &fsr, kLSRolesAll, kLSItemContentType, &tr );
#endif

	// does it look sufficiently like a CL Movie?
	if ( tr && CFStringGetTypeID() == CFGetTypeID( tr ) )
		{
		CFStringRef uti = static_cast<CFStringRef>( tr );
		bIsMovieFile = UTTypeConformsTo( uti, kUTTypeClanLordMovie );
		}
	if ( tr )
		CFRelease( tr );
	
	if ( bIsMovieFile )
		{
		if ( noErr == CCLMovie::StartReadMovie( &gDTSOpenFile ) )
			{
//			gDTSOpenFile.SetFileName( "" );
			PlayGame();
			}
		}
}


/*
**	OSNew
**
**	handle 'oapp' event
*/
void
OSNew()
{
	// look for the (new-style) auto-join parameter on the 'oapp' event
	// (see HandleRejoinEvent() for the rationale).
	// Don't bother if we don't have login credentials, or
	// if we're already playing (i.e. if this is actually a 'rapp' event not a 'oapp' one.)
	if ( gPrefsData.pdCharName[0]
	&&	 gPrefsData.pdAccount[0]
	&&   not gPlayingGame )
		{
		bool bAutoJoin = false;
		
		AppleEvent evt = { typeNull, nullptr };
		OSStatus err = AEGetTheCurrentEvent( &evt );
		__Check_noErr( err );
		if ( noErr == err )
			{
			// look for the Boolean flag that a prior client instance might have inserted
			// onto this 'oapp' event, via LSOpen()
			AERecord parms = { typeNull, nullptr };
			err = AEGetParamDesc( &evt, keyAEPropData, typeAERecord, &parms );
			if ( noErr == err )
				{
				err = AEGetParamPtr( &parms, kJoinGameEvent,
					typeBoolean, nullptr,
					&bAutoJoin, sizeof bAutoJoin, nullptr );
				}
			__Verify_noErr( AEDisposeDesc( &parms ) );
			}
		__Verify_noErr( AEDisposeDesc( &evt ) );
		
		if ( bAutoJoin )
			err = PerformAutoJoin();
		}
}


/*
**	DoRecordMovie()
*/
void
DoRecordMovie()
{
	if ( CCLMovie::IsRecording() )
		{
		CCLMovie::StopMovie();
		gDTSMenu->SetFlags( mRecordMovie, kMenuNormal );
		}
	else
		{
		if ( not gPlayingGame )
			InitGameState();
		
		// gPlayingGame = false;
		DTSError err = CCLMovie::StartRecordMovie();
		gDTSMenu->SetFlags( mRecordMovie, (noErr == err) ? kMenuChecked : kMenuNormal );
		}
}


/*
**	IsPrimaryWindow()
**
**	is this one of our 4 main windows (game, text, inventory, players)?
*/
static bool
IsPrimaryWindow( const DTSWindow * w )
{
	return w == gGameWindow ||
		   w == gTextWindow ||
		   w == gInvenWindow ||
		   w == gPlayersWindow;
}


/*
**	DoWindowCmd()
**
**	hand the menu command to the front window
*/
bool
DoWindowCmd( long menuCmd )
{
	bool result = false;
	if ( DTSWindow * front = gDTSApp->GetFrontWindow() )
		{
		if ( IsPrimaryWindow( front ) )
			result = front->DoCmd( menuCmd );
		}
	
	return result;
}


/*
**	CLApp::KeyStroke()
**
**	Handle key strokes by the user.
*/
bool
CLApp::KeyStroke( int ch, uint modifiers )
{
	// was it a macro? Stop now if so
	if ( DoMacroKey( &ch, modifiers ) )
		return true;
	
	// Or was it an inventory key?
	if ( InventoryShortcut( ch, modifiers ) )
		return true;
	
	// Maybe it was a window-specific key?
	bool bResult = false;
	if ( DTSWindow * front = GetFrontWindow() )
		{
		if ( IsPrimaryWindow( front ) )
			bResult = front->KeyStroke( ch, modifiers );
		}
	
	// Last resort, see if it was a menu key
	if ( not bResult )
		bResult = DTSApp::KeyStroke( ch, modifiers );
	
	return bResult;
}


/*
**	CLApp::Idle()
**
**	blink the caret
*/
void
CLApp::Idle()
{
	//	reset the cursor to nothing - the idle routines can change it
	int prevCursor = gCursor;
	gCursor = 0;
	
	// let the frontmost window get some idle time
	if ( DTSWindow * front = GetFrontWindow() )
		{
		if ( IsPrimaryWindow( front ) )
			if ( front )
				front->Idle();
		}
	
	// do the network i/o for this frame
	if ( gDoComm )
		DoComm();
	
#if USE_SPEECH
	// run the text-to-speech engine
	if ( gSpeechAvailable )
		IdleSpeech();
#endif	// USE_SPEECH
	
	//	apply cursor change
	//	flashes a little when user is typing from ObscureCursor() call
	//  (there's gotta be a better way to do this)
	if ( not gInBack )	// non-front apps shouldn't touch cursor
		{
		// override previously set cursors if using click-toggles mode
		if ( kMoveClickToggle == gPrefsData.pdMoveControls
		&&	 gClickState
		&&	 gPlayingGame
		&&	 not IsKeyDown( kMenuModKey )
		&&	 not IsKeyDown( kOptionModKey )
		&&	 not IsKeyDown( kControlModKey ) )
			{
			gCursor = 128;
			}
		
		if ( gCursor )
			{
			if ( gCursor != prevCursor )
				{
				if ( kLaunchURLHandler_PointCursorID == gCursor )
					::SetThemeCursor( kThemePointingHandCursor );
				else
					DTSShowCursor( gCursor );
				
				DTSShowMouse();
				}
			}
		else
			::SetThemeCursor( kThemeArrowCursor );
		}
}


/*
**	CLApp::Suspend()
**
**	we've moved to the background
*/
void
CLApp::Suspend()
{
	// force a reset of mouse-movement state
	gClickState = 0;
	
	// suspend the application
	gInBack = true;
	DTSApp::Suspend();
	
#if ENABLE_MUSIC_FILES
	// turn off the background music
	if ( not gMusicDisabled )
		PauseMusic();
#endif
	
#if USE_SPEECH
	SuspendSpeech();
#endif
}


/*
**	CLApp::Resume()
**
**	we've moved to the front
*/
void
CLApp::Resume()
{
	gInBack = false;
	
	// if there was an OS notification active, due to an
	// incoming thought message, kill it off now.
	KillNotification();
	
	DTSApp::Resume();
	
#if ENABLE_MUSIC_FILES
	if ( not gMusicDisabled )
		ResumeMusic();
#endif
	
	ResetDockIcon();
	
#if USE_SPEECH
	ResumeSpeech();
#endif
}


/*
**	CLApp::Click()
**
**	assume we did not click in the game window
*/
bool
CLApp::Click( const DTSPoint& loc, ulong time, uint modifiers )
{
	gLastClickGame = false;
	
	return DTSApp::Click( loc, time, modifiers );
}


/*
**	CLApp::ProcessCommand()
**
**	deal with user commands (such as menu selections)
*/
OSStatus
CLApp::ProcessCommand( const HICommandExtended& cmd, CarbonEvent& evt )
{
	if ( kHICommandPreferences == cmd.commandID )
		{
		GetMainPrefs();
		return noErr;
		}
	
	if ( cmd_ClientHelpURL == cmd.commandID )
		{
		gURLMenu.DoCmd( mClientHelpURL /* cmd.commandID */ );
		return noErr;
		}
	
	OSStatus result = eventNotHandledErr;
	
	if ( cmd.attributes & kHICommandFromMenu )
		{
		// we could also process not-from-menu commands, based on cmd.commandID,
		// but we're not really set up for that (apart from prefs: vide supra)
		MenuID menuID = ::GetMenuID( cmd.source.menu.menuRef );
		int menuCmd = MakeLong( menuID, cmd.source.menu.menuItemIndex );
		
		// for help menu, adjust the command so any system-provided items
		// are properly ignored
		if ( kHMHelpMenuID == menuID )
			menuCmd -= gDTSMenu->GetHelpMenuOffset();
		
		bool bHandled = gDTSMenu->DoCmd( menuCmd );
		if ( bHandled )
			result = noErr;
		}
	
	if ( eventNotHandledErr == result )
		result = HICommandResponder::ProcessCommand( cmd, evt );
	
	return result;
}


/*
**	CLApp::UpdateCommandStatus()
**
**	make the menu items match internal reality
*/
OSStatus
CLApp::UpdateCommandStatus( const HICommandExtended& cmd, CarbonEvent& evt,
		EState& oState, CFStringRef& oText ) const
{
	OSStatus result = noErr;
	oState = enable_LEAVE_ALONE;
	
	// mostly just placeholders for now. as these get fleshed out, don't forget
	// to set the corresponding menu items as "auto-disable" in the nib
	switch ( cmd.commandID )
		{
		// File menu
		case cmd_CharacterMgr:
		case cmd_SelectChar:
		case cmd_JoinGame:
			oState = (gPlayingGame || gConnectGame) ? enable_NO : enable_YES;
			break;
		
		case cmd_LeaveGame:
			oState = gPlayingGame ? enable_YES : enable_NO;
			break;
		
		case cmd_ViewMovie:
			oState = (gConnectGame || gPlayingGame) ? enable_NO : enable_YES;
			__Verify_noErr( SetMenuCommandMark( nullptr, cmd.commandID,
								CCLMovie::IsReading() ? kMenuCheckmarkGlyph : kMenuNullGlyph ) );
			break;
		
		case cmd_RecordMovie:
			{
			// "Record Movie" should generally always be enabled, unless...
			if ( gConnectGame				// still not fully connected to server, or
			||   not gPlayingGame			// not playing, or
			||   CCLMovie::IsReading()		// in the midst of playback, or
			||   (CCLMovie::HasMovie() && not CCLMovie::IsRecording()) )
				{							// we somehow have an active movie file that
				oState = enable_NO;			// is NOT already being written to.
				}
			else
				oState = enable_YES;		// otherwise, it's OK to record
			
			// checked iff currently recording
			__Verify_noErr( SetMenuCommandMark( nullptr, cmd.commandID,
								CCLMovie::IsRecording() ? kMenuCheckmarkGlyph : kMenuNullGlyph ) );
			}
			break;
		
#if 0
		case cmd_FindText:
		case cmd_FindAgain:
			// ask text window
			break;
		
		// options menu
		case cmd_Sound:
		case cmd_TextStyles:
		case cmd_SaveMsgLog:
		case cmd_NetworkStats:
		case cmd_GameWindow:
		case cmd_TextWindow:
		case cmd_InventoryWindow:
		case cmd_PlayersWindow:
		
		// commands menu
		case cmd_Info:
		case cmd_Pull:
		case cmd_Push:
		case cmd_Karma:
		case cmd_Thank:
		case cmd_Curse:
//		case cmd_Share:
//		case cmd_Unshare:
		
		// help menu
		case cmd_ClientHelpURL:
			oState = enable_YES;
			break;
		
# ifdef DEBUG_VERSION
		// God menu
		case cmd_HideMonsterNames:	// mShowMonsterNames
		case cmd_Account:			// mAccount
		case cmd_Set:				// mSet
		case cmd_Stats:				// mStats
		case cmd_Ranks:				// mRanks
		case cmd_Goto:				// mGoto
		case cmd_Where:				// mWhere
		case cmd_Punt:				// mPunt
		case cmd_Tricorder:			// mTricorder
		case cmd_Follow:			// mFollow
		
		// Debug menu
		case cmd_ShowFrameRate:		// mShowFrameTime
		case cmd_SaveMsgLog:		// mSaveMessageLog
		case cmd_ShowDrawTime:		// mShowDrawTime
		case cmd_ShowImageCount:	// mShowImageCount
		case cmd_FlushCaches:		// mFlushCaches
		case cmd_SetIPAddress:		// mSetIPAddress
		case cmd_FastBlend:			// mFastBlitters
		case cmd_SecretDrawOpts:	// mDrawRateControls
		case cmd_DumpBlocks:		// mDumpBlocks
		case cmd_PlayerFileStats:	// mPlayerFileStats
		case cmd_SetVersionNumber:	// mSetVersionNumbers
			oState = enable_YES;
			break;
# endif  // DEBUG_VERSION
#endif  // 0
		
		default:
			result = eventNotHandledErr;
		}
	
	if ( eventNotHandledErr == result )
		result = HICommandResponder::UpdateCommandStatus( cmd, evt, oState, oText );
	
	return result;
}


/*
**	CLApp:HandleEvent()
**
**	for now, just handle scroll wheel events
**	and commands e.g. from menus
*/
OSStatus
CLApp::HandleEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	UInt32 clas = evt.Class();
	UInt32 kind = evt.Kind();
	OSStatus err;
	
	if ( kClientAppSign == clas
	&&   kEvent_ViewMovie == kind )
		{
		CFTypeRef tr = nullptr;
		err = evt.GetParameter( kEventParamDirectObject, typeCFTypeRef, sizeof tr, &tr );
		if ( noErr == err && tr && CFURLGetTypeID() == CFGetTypeID( tr ) )
			{
			CFURLRef url = static_cast<CFURLRef>( tr );
			err = DidChooseMovieFile( url );
			}
		
		if ( noErr == err )
			result = noErr;
		}
	
	// propagate to superclass
	if ( eventNotHandledErr == result )
		result = HICommandResponder::HandleEvent( evt );
	
	return result;
}


/*
**	CLApp::Action()
**
**	Navigation Services support.
**	right now, the only Get/Put-File call in the entire client is for "View Movie..."
**	So when we get a 'choose' nav-action, we know the user chose a movie, so we go show it.
*/
void
CLApp::Action( NavCBRec * parms )
{
	if ( kNavUserActionChoose == parms->userAction
	||   kNavUserActionOpen   == parms->userAction )
		{
		// fish out the reply
		NavReplyRecord reply;
		OSStatus err = NavDialogGetReply( parms->context, &reply );
		__Check_noErr( err );
		if ( noErr == err )
			{
			// they didn't cancel?
			if ( reply.validRecord )
				{
				// fish out the chosen file, as a URL
				CFURLRef url = nullptr;
				uchar buff[ PATH_MAX ];
				Size len = 0;
				err = AEGetNthPtr( &reply.selection, 1, typeFileURL,
						nullptr, nullptr, buff, sizeof buff, &len );
				if ( noErr == err && len <= static_cast<Size>( sizeof buff ) )
					{
					url = CFURLCreateWithBytes( kCFAllocatorDefault,
							buff, len, kCFStringEncodingUTF8, nullptr );
					}
				
				// ... and play it!
				if ( noErr == err && url )
					{
#if 0
					err = DidChooseMovieFile( url );
#else
					CarbonEvent evt( kClientAppSign, kEvent_ViewMovie );
					err = evt.SetParameter( kEventParamDirectObject,
									typeCFTypeRef, sizeof url, &url );
					if ( noErr == err )
						err = evt.Post( nullptr, GetMainEventQueue(), kEventPriorityLow );
#endif
					}
				if ( url )
					CFRelease( url );
				}
			
			// we're done with the reply now
			__Verify_noErr( NavDisposeReply( &reply ) );
			}
		}
}


/*
**	CLApp::CopyFilterTypeIdentifiers()
**
**	NavSrvHandler support -- returns the array of UTIs that are acceptable
**	for a choose-file operation
*/
CFArrayRef
CLApp::CopyFilterTypeIdentifiers() const
{
	// only wanna open movies
	CFTypeRef myType = kUTTypeClanLordMovie;
	return CFArrayCreate( kCFAllocatorDefault, &myType, 1, &kCFTypeArrayCallBacks );
}


/*
**	GetPlayerInput()
**
**	collect the draw state data from the host
**	and send back the player input
*/
void
GetPlayerInput( PlayerInput * pi )
{
	DTSPoint pt( 0, 0 );
	uint flags = 0;
	
	switch ( gPrefsData.pdMoveControls )
		{
		case kMoveClickHold:
			GetGWMouseLoc( &pt );
			if ( gLastClickGame
			&&   IsButtonStillDown() )
				{
				flags = gClickFlag;
				}
			else
				gLastClickGame = false;
			break;
		
		case kMoveClickToggle:
			GetGWMouseLoc( &pt );
			if ( gClickState
			&&	 not IsKeyDown( kMenuModKey )
			&&	 not IsKeyDown( kOptionModKey )
			&&	 not IsKeyDown( kControlModKey ) )
				{
				flags = kPIMDownField;
				}
			break;
		}
	
	// cache the string
	ulong prevCmdNum = gCommandNumber;
	GetSendString( pi->piKeyString, sizeof pi->piKeyString );
	
	// record the time we send this command to server
	if ( gCommandNumber != prevCmdNum )
		gLatencyRecorder.RecordSentCmd( gCommandNumber );
	
	// clear the inventory command
	ClearInvCmd();
	
	pi->piMouseH		= NativeToBigEndian( pt.ptH );	// always send mouse now
	pi->piMouseV		= NativeToBigEndian( pt.ptV );
	pi->piFlags			= NativeToBigEndian( 0 );
	
	if ( flags )
		pi->piFlags		= NativeToBigEndian( (uint16_t) flags );
	else
		{
		if ( gMovePlayer )
			{
			// keyboard/macro move
			pi->piMouseH = NativeToBigEndian( gClickLoc.ptH );
			pi->piMouseV = NativeToBigEndian( gClickLoc.ptV );
			pi->piFlags  = NativeToBigEndian( (uint16_t) kPIMDownField );
			
			if ( not gClickLoc.ptH && not gClickLoc.ptV )
				gMovePlayer = false;	// stop the guy, and stop moving :
			}
		}
	
	// init the playerinput record
	pi->piMsgTag      = NativeToBigEndian( (uint16_t) kMsgPlayerInput );
	pi->piAckFrame    = NativeToBigEndian( gAckFrame );
	pi->piResendFrame = NativeToBigEndian( gResendFrame );
	pi->piCommandNum  = NativeToBigEndian( gCommandNumber );
}


#pragma mark -
/*
**	APP PREFERENCES
*/

#if 0	// we no longer try to update prefs this old
#pragma pack( push, 2 )

struct PrefsData_40
{
	int32_t			pdVersion;					// prefs version number				g + c
	
	DTSRect			pdGWPos;					// game window position				c
	DTSRect			pdTextPos;					// text window position				c
	DTSRect			pdInvenPos;					// inventory window position		c
	DTSRect			pdPlayersPos;				// players window position			c
	DTSPoint		pdMsgPos;					// message window position			c
	
	int32_t			pdShowNames;				// show player/npc names			c
	int32_t			pdGodShowMonsterNames;		// Show names for monsters			c
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
	
	CLStyleRecord	pdStyles[ 14 ];				// misc text styles					c
	
	int32_t			pdMaxNightPct;				// max %age night displayed			c
	int32_t			pdBubbleBlitter;			// blitter used for normal bubbles	c
	int32_t			pdFriendBubbleBlitter;		// blitter used for friend bubbles	c
	
// v30
	DTSBoolean		pdSpeaking[ 14 ];			// speak this msg aloud?			c
	
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
};

#pragma pack( pop )
#endif  // 0


#if USE_CF_PREFERENCES
/*
**	CF Preference keys
**	remapping CL Prefs to CFPreferences: naming ideas
*/

#define pref_GameWinPos			CFSTR("GameWindowPosition")		// rects
#define pref_TextWinPos			CFSTR("TextWindowPosition")
#define pref_InvenWinPos		CFSTR("InventoryWindowPosition")
#define pref_PlayersWinPos		CFSTR("PlayersWindowPosition")
//#define pref_DebugWinPos		CFSTR("DebugWindowPosition")	// Message window

#define pref_Sounds				CFSTR("EnableSounds")			// bool: pdSound
#define pref_SoundVolume		CFSTR("SoundVolume")			// integer: pdSoundVolume
#define pref_BardVolume			CFSTR("BardMusicVolume")		// integer: pdBardVolume

#define pref_ShowNames			CFSTR("ShowMobileNames")		// bool: pdShowNames
#define pref_GMHideMonsterNames	CFSTR("GMHideMonsterNames")		// bool: pdGodHideMonsterNames
#define pref_TimeStamp			CFSTR("ShowTimeStamps")			// bool: pdTimeStamp
#define pref_BrightColors		CFSTR("BrightColors")			// bool: pdBrightColors
#define pref_MoveControls		CFSTR("ClickMoveMode")			// integer: pdMoveControls
#define pref_SuppressClanning	CFSTR("SuppressClanningMessages")	// bool: pdSuppressClanning
#define pref_SuppressFallen		CFSTR("SuppressFallenMessages")		// bool: pdSuppressFallen
#define pref_NoSuppressFriends	CFSTR("DontSuppressFriends")	// bool: pdNoSuppressFriends
#define pref_TreatSharesAsFriends CFSTR("TreatSharesAsFriends")	// bool: pdTreatSharesAsFriends

// we used to have 1 bool that controlled saving both character- and account passwords;
// but henceforth that task is split in two
#define pref_SaveCharPass		CFSTR("SaveCharacterPassword")	// bool: pdSavePassword
#define pref_SaveAcctPass		CFSTR("SaveAccountPassword")	// bool: <no equivalent>
#define pref_AcctName			CFSTR("AccountName")			// string: pdAcctName
#define pref_CharName			CFSTR("CharacterName")			// string: pdCharName
	// these are now in keychain, not prefs
// #define pref_AcctPass			CFSTR("AccountPassword")		// string: pdAcctPass
// #define pref_CharPass			CFSTR("CharacterPassword")		// string: pdCharPass

#define pref_ShowFrameTime		CFSTR("DebugShowFrameTimes")	// bool: pdShowFrameTime
#define pref_SaveMsgLog			CFSTR("DebugSaveMessageLog")	// bool: pdSaveMsgLog
#define pref_ShowDrawTime		CFSTR("DebugShowDrawTimes")		// bool: pdShowDrawTime
#define pref_ShowImageCount		CFSTR("DebugShowImageCounts")	// bool: pdShowImageCount

#define pref_SaveTextLog		CFSTR("SaveTextLog")			// bool: pdSaveTextLog
#define pref_NewLogEveryJoin	CFSTR("NewLogEveryJoin")		// bool: pdNewLogEveryJoin
#define pref_NoMovieTextLogs	CFSTR("NoMovieTextLogs")		// bool: pdNoMovieTextLogs

#define pref_HostAddr			CFSTR("ServerAddress")			// string: pdHostAddr
#define pref_AltHostAddr		CFSTR("AltServerAddress")		// string: pdProxyAddr
#define pref_ClientPort			CFSTR("ClientPort")				// integer: pdClientPort
#define pref_UseSpecificPort	CFSTR("UseSpecificPort")		// bool: pdUseArbitrary

#define pref_TextStyles			CFSTR("TextStyles")				// array: pdStyles & pdSpeaking

#define pref_MaxNightLevel		CFSTR("MaxNightLevel")			// integer: pdMaxNightPct
#define pref_BubbleOpacity		CFSTR("BubbleOpacity")			// integer: pdBubbleBlitter
#define pref_FriendBubbleOpacity CFSTR("FriendBubbleOpacity")	// integer: pdFriendBubbleBlitter

#define pref_HBColorStyle		CFSTR("HealthBarStyle")			// integer: pdHBColorStyle
#define pref_HBPlacement		CFSTR("HealthBarPlacement")		// integer: pdHBPlacement

#define pref_ThinkNotify		CFSTR("NotifyForThinkto")		// bool: pdThinkNotify
#define pref_DisableCmdQ		CFSTR("DisableCmdQ")			// bool: pdDisableCmdQ

#define pref_NumFrames			CFSTR("FramesPlayed")			// integer: pdNumFrames
#define pref_LostFrames			CFSTR("FramesLost")				// integer: pdLostFrames

// do we need these 2?
#define pref_ClientVersion		CFSTR("ClientVersion")			// integer: pdClientVersion
#define pref_CompiledClient		CFSTR("CompiledClientVersion")	// integer: pdCompiledClient

#define pref_OpenGLEnable		CFSTR("UseOpenGL")				// bool: pdOpenGLEnable
#define pref_OpenGLEffects		CFSTR("UseGLEffects")			// bool: pdOpenGLEnableEffects
#define pref_OpenGLRenderer		CFSTR("OpenGLRenderer")			// integer: pdOpenGLRenderer

#define pref_InventoryShortcut	CFSTR("InventoryItemQuickKeys")
		// dictionary { itemID (&) itemIndex -> character [0-9] }


/*
**	Some local additions to the "Prefs" namespace
*/
namespace Prefs
{

/*
**	Prefs::Set( DTSRect )
*/
void
Set( CFStringRef key, const DTSRect& r )
{
	CGRect r2 = CGRectMake( r.rectLeft, r.rectTop,
							r.rectRight - r.rectLeft, r.rectBottom - r.rectTop );
	Set( key, r2 );
}


/*
**	Prefs:Set( const char * someString )
*/
void
Set( CFStringRef key, const char * s )
{
	if ( CFStringRef s2 = CreateCFString( s ) )
		{
		Set( key, s2 );
		CFRelease( s2 );
		}
}


/*
**	Prefs::GetDTSRect()
*/
const DTSRect
GetDTSRect( CFStringRef key )
{
	CGRect r = GetRect( key, CGRectZero );
	return DTSRect( CGRectGetMinX( r ), CGRectGetMinY( r ),
					CGRectGetMaxX( r ), CGRectGetMaxY( r ) );
}


/*
**	Prefs::GetString() -- fills a character buffer supplied by caller
*/
void
GetString( CFStringRef key, char * dst, size_t dstsz,
			CFStringEncoding enc = kCFStringEncodingMacRoman )
{
	bool got = false;
	
	if ( CFTypeRef v = Prefs::Copy( key ) )
		{
		if ( CFStringGetTypeID() == CFGetTypeID( v ) )
			{
			CFStringRef s = static_cast<CFStringRef>( v );
			got = CFStringGetCString( s, dst, dstsz, enc );
			}
		CFRelease( v );
		}
	if ( not got )
		if ( dst && dstsz > 0 )
			dst[ 0 ] = '\0';
}

}	// end namespace Prefs

// extern
void	SaveTextStylesToPrefs();
void	GetTextStylesFromPrefs();
void	SaveInventoryShortcutKeys();
void	GetInventoryShortcutKeys();

#endif  // USE_CF_PREFERENCES


/*
**	UpdatePrefs()
**
**	attempt to update the prefs from whatever version they are,
**	to the most recent version
*/
static DTSError
UpdatePrefs()
{
	DTSError result = noErr;
	//if ( noErr == result )
		{
		switch ( gPrefsData.pdVersion )
			{
			// initialize everything
			case -1:
			case 0:
//				memset( &gPrefsData, 0, sizeof gPrefsData );
				
				// ... but DON'T erase the default text styles!
				memset( &gPrefsData, 0, offsetof( PrefsData, pdStyles ) );
				memset( &gPrefsData.pdMaxNightPct, 0,
					sizeof(PrefsData) - offsetof( PrefsData, pdMaxNightPct ) );
				
				gPrefsData.pdGWPos		= gDefaultWindowPositions[ kGameWindow ];
				gPrefsData.pdTextPos	= gDefaultWindowPositions[ kTextWindow ];
				gPrefsData.pdInvenPos	= gDefaultWindowPositions[ kInvenWindow ];
				gPrefsData.pdPlayersPos	= gDefaultWindowPositions[ kPlayersWindow ];
				gPrefsData.pdMsgPos.Set( 50, 50 + 400 );
//				gPrefsData.pdShowNames	= false;			// actually controls "auto-hide"
//				gPrefsData.pdGodHideMonsterNames = false;
				gPrefsData.pdSound		= true;
//				gPrefsData.pdTimeStamp	= false;
				
				// Set default size to large
				gPrefsData.pdLargeWindow	= true;
				
//				gPrefsData.pdBrightColors	= false;
				gPrefsData.pdMoveControls   = kMoveClickHold;
//				gPrefsData.pdSuppressClanning = 0;
//				gPrefsData.pdSuppressFallen = 0;
				gPrefsData.pdMusicPlay      = true;
				gPrefsData.pdMusicVolume    = 80;
//				gPrefsData.pdSavePassword   = false;
//				gPrefsData.pdAccount[0]     = 0;
//				gPrefsData.pdAcctPass[0]    = 0;
//				gPrefsData.pdCharName[0]    = 0;
//				gPrefsData.pdCharPass[0]    = 0;
//				gPrefsData.pdShowFrameTime  = false;
//				gPrefsData.pdSaveMsgLog     = false;
//				gPrefsData.pdSaveTextLog    = false;
//				gPrefsData.pdShowDrawTime   = false;
//				gPrefsData.pdShowImageCount = false;
//				gPrefsData.pdFastBlitters   = false;
				StringCopySafe( gPrefsData.pdHostAddr,
					kDefaultHostAddrInternet, sizeof(gPrefsData.pdHostAddr)  );
				StringCopySafe( gPrefsData.pdProxyAddr,
					kDefaultHostAddrInternet, sizeof(gPrefsData.pdProxyAddr) );
				for ( int n = kMsgDefault; n < kMsgMaxStyles; ++n )
					{
					SetUpStyle( static_cast<MsgClasses>(n), &gPrefsData.pdStyles[n] );
//					gPrefsData.pdSpeaking[n] = false;
					}
				gPrefsData.pdMaxNightPct = 100;
				gPrefsData.pdBubbleBlitter = kBlitterTransparent;
				gPrefsData.pdFriendBubbleBlitter = kBlitterTransparent;
				gPrefsData.pdItemPrefsVers = kItemPrefsVersion;
//				gPrefsData.pdHBColorStyle = 0;
//				gPrefsData.pdHBPlacement = 0;
//				gPrefsData.pdThinkNotify = 0;
//				gPrefsData.pdDisableCmdQ = 0;
				gPrefsData.pdNoSuppressFriends = 1;
				gPrefsData.pdUseArbitrary = true;
				gPrefsData.pdClientPort   = 5000;
//				gPrefsData.pdTreatSharesAsFriends = 0;
				gPrefsData.pdLanguageID = kRealLangDefault;
				
				gPrefsData.pdSoundVolume = 100;
				gPrefsData.pdClientVersion = kFullVersionNumber;
				gPrefsData.pdCompiledClient = kFullVersionNumber;
				gPrefsData.pdOpenGLEnable = 1;
				gPrefsData.pdOpenGLEnableEffects = 1;
//				gPrefsData.pdOpenGLRenderer = 0;
				gPrefsData.pdBardVolume = 100;
				
				// fall through to the incremental changes...
				FALLTHRU_OK;
				
			// incremental changes in each pref revision
#if 0
	/*
	**	for prefsVersion 41, the size of the pdStyles array grew, which means that
	**	all the offsets used by the following incremental updates are off, rendering
	**	them useless. So they are ifdef'ed out, and we start a new generation of
	**	prefs incremental updates with version 42.
	*/

			case 27:
				// added text styles
				for ( int n = kMsgDefault; n < kMsgLastMsg; ++n )
					SetUpStyle( n, &gPrefsData.pdStyles[n] );
			
			case 28:
				// added max night percentage
				gPrefsData.pdMaxNightPct = 100;
				gPrefsData.pdBubbleBlitter = kBlitterTransparent;
				gPrefsData.pdFriendBubbleBlitter = kBlitterTransparent;
			
			case 29:
				// added speakable messages
				for ( int n = kMsgDefault; n < kMsgLastMsg; ++n )
					gPrefsData.pdSpeaking[n] = false;
			
			case 30:
				// added translucent health bar thingies, changed item prefs
				gPrefsData.pdHBColorStyle = 0;	// by type
				gPrefsData.pdHBPlacement = 0;	// bottom

				// ( the change from v30 to v31 redefined the old pdHBEnabled boolean to
				// the new pdItemPrefsVers, and so we need to reset it.)

			case 31:
				gPrefsData.pdItemPrefsVers = 0;	// flag need rebuild
			
			case 32:
				gPrefsData.pdThinkNotify = 0;	// no notifications
			
			case 33:
				// as of v34, the passwords are SimpleEncrypt()ed before being
				// stored, or zapped to zeros if the player doesn't want 'em saved.
				// so for the 33->34 transition we must encrypt them one time
				SimpleEncrypt( gPrefsData.pdAcctPass, sizeof( gPrefsData.pdAcctPass ) );
				SimpleEncrypt( gPrefsData.pdCharPass, sizeof( gPrefsData.pdCharPass ) );
			
			case 34:
				gPrefsData.pdDisableCmdQ = 0;	// cmd-q exits
			
			case 35:
				// default host address became a DNS name instead of a raw IP address
				StringCopySafe( gPrefsData.pdHostAddr,
					kDefaultHostAddrInternet, sizeof(gPrefsData.pdHostAddr) );
			
			case 36:
				// added pdNoSuppressFriends
				gPrefsData.pdNoSuppressFriends = 1;
			
			case 37:
				// added proxy
				gPrefsData.pdUseArbitrary = true;
				gPrefsData.pdClientPort   = 5000;
			
			case 38:
				// changed encryption
				SimpleEncrypt( gPrefsData.pdAcctPass, sizeof( gPrefsData.pdAcctPass ) );
				SimpleEncrypt( gPrefsData.pdCharPass, sizeof( gPrefsData.pdCharPass ) );
				SimpleEncrypt2( kClientPrefsFName,
					gPrefsData.pdAcctPass, sizeof( gPrefsData.pdAcctPass ) );
				SimpleEncrypt2( kClientPrefsFName,
					gPrefsData.pdCharPass, sizeof( gPrefsData.pdCharPass ) );
			
			case 39:
				// added pdTreatSharesAsFriends
				gPrefsData.pdTreatSharesAsFriends = false;
			
				// we're falling through from case 0
				// yeah i know this is wacky
				// someone have a better way?
				if ( 0 )
					{
			case 40:
					const int kMsgLastMsg_40 = 14;		// the old size
					
					// first, relocate the chunk from pdItemPrefsVers thru pdTreatSharesAsFriends
					const PrefsData_40 * oldprefs = (const PrefsData_40 *) &gPrefsData;
					
					// move this much data
					ptrdiff_t size = (char *) &oldprefs->pdTreatSharesAsFriends
						- (char *) &oldprefs->pdItemPrefsVers
						+ sizeof( oldprefs->pdTreatSharesAsFriends );
					
					// shove it downwards
					memmove( &gPrefsData.pdItemPrefsVers, &oldprefs->pdItemPrefsVers, size );
					
					// repeat for the chunk from pdMaxNightPct thru pdSpeaking[ kMsgLastMsg_40 ];
					size = (char *) &oldprefs->pdItemPrefsVers
						 - (char *) &oldprefs->pdMaxNightPct;
					memmove( &gPrefsData.pdMaxNightPct, &oldprefs->pdMaxNightPct, size );
					
					// now initialize the newly added style-array entries
					for ( int ii = kMsgLastMsg_40;  ii < kMsgMaxStyles;  ++ii )
						{
						CLStyleRecord & p = gPrefsData.pdStyles[ ii ];
						p.font = 1;
						p.size = 10;
						p.face = 0;
						p.color.SetBlack();

						gPrefsData.pdSpeaking[ ii ] = false;
						}
					
					// then remap the style entries that moved
					for ( int ii = kMsgLastMsg_40 - 1; ii > kMsgMySpeech; --ii )
						{
						gPrefsData.pdStyles[ ii ] = gPrefsData.pdStyles[ ii - 1 ];
						gPrefsData.pdSpeaking[ ii ] = gPrefsData.pdSpeaking[ ii - 1 ];
						}
					
					// and lastly, set up the new styles
					void * styleData = nullptr;
					size_t styleSize = 0;
					CLStyleRecord & p = gPrefsData.pdStyles[ kMsgMySpeech ];
					GetAppData( 'TxSt', 128 + kMsgMySpeech, &styleData, &styleSize );
					if ( sizeof(p) == styleSize )
						{
						memcpy( &p, styleData, styleSize );
						}
					else
						{
						p.face = 0;
						p.color.SetBlack();
						}
					p.font = 1;
					p.size = 12;
					delete[] static_cast<char *>( styleData );
					
					gPrefsData.pdSpeaking[ kMsgMySpeech ] = false;
					}
			
			case 41:
				// added sound volume
				gPrefsData.pdSoundVolume = 100;
			
			case 42:
				// added survey data. assume they haven't taken it.
				gPrefsData.pdSurveys = 0;
			
			case 43:
				// added num frames and lost frames.
				gPrefsData.pdNumFrames  = 0;
				gPrefsData.pdLostFrames = 0;
			
			case 44:
				// added client version number
				gPrefsData.pdClientVersion = kFullVersionNumber;
			
			case 45:
				// added client compiled version number
				gPrefsData.pdCompiledClient = kFullVersionNumber;

			case 46:
				// added OpenGL prefs
				gPrefsData.pdOpenGLEnable = 1;			// master enable ON
				gPrefsData.pdOpenGLEnableEffects = 1;	// ogl-specific effects ON
				gPrefsData.pdOpenGLRenderer = 0;		// let client pick renderer

			case 47:
				// added new log for each join and movie text logs
				gPrefsData.pdNewLogEveryJoin = 0;
				gPrefsData.pdNoMovieTextLogs = 0;
			
			case 48:
				// added bard volume
				gPrefsData.pdBardVolume = 100;			// 100%
			
			case 49:
				// added language ID (ignored if ! MULTILINGUAL)
# ifdef CLT_LANG
				if ( CLT_LANG >= kRealLangFirstAllowed )
					// our default language as defined by the client compile language
					gPrefsData.pdLanguageID = CLT_LANG;
				else
					// even the English client uses at least German
				gPrefsData.pdLanguageID = kRealLangDefault;
# else
				gPrefsData.pdLanguageID = kRealLangDefault;
# endif  // CLT_LANG
			
			case 50:
				// changed the sense of the erstwhile "pdGodShowMonsterNames" field:
				// it is now called pdGodHideMonsterNames.
				// We're re-forcing it to false again, so as of v418, GMs initially see monsters.
				// If they dislike that, they can toggle the pref w/the debug client;
				// future clients (even regular ones) will respect the setting.
				// However, if they do this, they'll be missing out on monsters whose names
				// are forcibly made visible (mobNameState = kNameStateShown) by scripts.
				gPrefsData.pdGodHideMonsterNames = false;
				FALLTHRU_OK;

#endif	// end of old pref modifications
			
			case 51:
				// no change in the prefs structure, but .pdAcctPass and .pdCharPass
				// get migrated into keychain
				MovePWsToKeychain();
				
	/*
	**	Future cases go here
	*/

				break;

			default:
				// any other prefs version, we can't handle
				result = -1;
				break;
			}
		}
	
	if ( noErr == result )
		gPrefsData.pdVersion = kPrefsVersion;
	
	return result;
}


const char gValidChars[] =
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		"~!@#$%^&*()_+`1234567890-=[]{}\\|;':<>?,./ ";


#if 0  // obsolete, now that passwords are kept in the Keychain
/*
**	SimpleEncrypt2()
**
**	encrypt the password using the file ID
**	minor inconvenience to folks that copy their prefs file
**	goes a long way towards defeating the strategy of stealing prefs files:
**	if the fileID is unaltered, this encryption is easily reversed; but if it changes
**	(e.g. by the Prefs file being copied or moved), then all you get is gibberish.
*/
static void
SimpleEncrypt2( const char * filename, char * text, size_t size )
{
	// convert the text to a Pascal string
//	::CopyCStringToPascal( text, (uchar *) text );
	size_t len = strlen( text );
	memmove( text + 1, text, len );
	text[ 0 ] = char( len );
	
	// pad the remainder text buffer with random characters
	for ( ++len;  len < size;  ++len )
		{
		uint r = GetRandom( sizeof(gValidChars) - 1 );
		text[ len ] = gValidChars[ r ];
		}
	
	// now simple-encrypt the whole thing
	SimpleEncrypt( text, size );
	
	// reseed the RNG using the file ID
	ulong save = GetRandSeed();
	uint32_t mask = MyGetFileID( filename );
	SetRandSeed( mask );
	
	// now xor the whole mess with "random" bytes, derived from the file ID
	uchar * ptr = (uchar *) text;
	for ( size_t sz = size;  sz > 0;  --sz )
		*ptr++ ^= GetRandom( 254 ) + 1;
	
	// restore the RNG seed
	SetRandSeed( save );
}
#endif  // 0


/*
**	SimpleDecrypt2()
**
**	inverse of the above: decrypt the password using the file ID
**	minor inconvenience to folks that copy their prefs file
**	goes a long way towards defeating the strategy of stealing prefs files
*/
static void
SimpleDecrypt2( const char * filename, char * text, size_t size )
{
	// reseed the RNG, using the file's ID
	ulong save = GetRandSeed();
	uint32_t mask = MyGetFileID( filename );
	SetRandSeed( mask );
	
	// now xor the whole mess with "random" bytes, derived from the ID
	uchar * ptr = (uchar *) text;
	for ( size_t sz = size;  sz > 0;  --sz )
		*ptr++ ^= GetRandom( 254 ) + 1;
	
	// restore the RNG seed
	SetRandSeed( save );
	
	// now simple-encrypt it (this is symmetric, so it's really "simple-DEcrypt")
	SimpleEncrypt( text, size );
	
	// sanity check the length
	size_t len = * (uchar *) text;
	if ( len >= size )
		{
		// something went wrong
		// forget the password
		*text = '\0';
		}
	else
		{
		// convert the text back into a C string
//		::CopyPascalStringToC( (uchar *) text, text );
		memmove( text, text + 1, len );
		text[ len ] = '\0';
		}
}


/*
**	MyGetFileID()
**
**	return the file ID
**	create one if necessary
*/
SInt32
MyGetFileID( const char * filename )
{
	SInt32 nodeID = 0x0FE1FDE1;
	
	DTSFileSpec dfs;
	dfs.SetFileName( filename );
	FSRef fsr;
	OSStatus err = DTSFileSpec_CopyToFSRef( &dfs, &fsr );
	if ( noErr == err )
		{
		FSCatalogInfo info;
		memset( &info, 0, sizeof info );
		err = FSGetCatalogInfo( &fsr, kFSCatInfoNodeID, &info, nullptr, nullptr, nullptr );
		if ( noErr == err )
			nodeID = info.nodeID;
		}
	return nodeID;
}


#if USE_CF_PREFERENCES
/*
**	LoadPrefsAux()
**
**	read the saved gPrefs data, via CFPreference
*/
void
LoadPrefsAux()
{
	// ensure CFPreferences are loaded
//	(void) CFPreferencesAppSynchronize( kCFPreferencesCurrentApplication );
	
#define Get1Rect( key, field )		gPrefsData.field = Prefs::GetDTSRect( pref_ ## key )
	Get1Rect( GameWinPos, pdGWPos );
	Get1Rect( TextWinPos, pdTextPos );
	Get1Rect( InvenWinPos, pdInvenPos );
	Get1Rect( PlayersWinPos, pdPlayersPos );
#undef Get1Rect
	
#define Get1Bool( key, field )		gPrefsData.field = Prefs::GetBool( pref_ ## key )
#define Get1Int( key, field )		gPrefsData.field = Prefs::GetInt( pref_ ## key )
#define Get1String( key, field ) \
			Prefs::GetString( pref_ ## key, gPrefsData.field, sizeof gPrefsData.field )

	Get1Bool( Sounds,				pdSound );
	Get1Int( SoundVolume,			pdSoundVolume );
	Get1Int( BardVolume,			pdBardVolume );
	Get1Bool( ShowNames,			pdShowNames );
	Get1Bool( GMHideMonsterNames,	pdGodHideMonsterNames );
	Get1Bool( TimeStamp,			pdTimeStamp );
	Get1Bool( BrightColors,			pdBrightColors );
	Get1Int( MoveControls,			pdMoveControls );
	Get1Bool( SuppressClanning,		pdSuppressClanning );
	Get1Bool( SuppressFallen,		pdSuppressFallen );
	Get1Bool( NoSuppressFriends,	pdNoSuppressFriends );
	Get1Bool( TreatSharesAsFriends, pdTreatSharesAsFriends );
	
	Get1Bool( SaveCharPass,			pdSavePassword );
	Get1Bool( SaveAcctPass,			pdSavePassword );	// FIXME
	Get1String( AcctName,			pdAccount );
	Get1String( CharName,			pdCharName );
#if 0  // now in keychain
	Get1String( AcctPass,			pdAcctPass );
	Get1String( CharPass,			pdCharPass );
#endif
	
	Get1Bool( ShowFrameTime,		pdShowFrameTime );
	Get1Bool( SaveMsgLog,			pdSaveMsgLog );
	Get1Bool( ShowDrawTime,			pdShowDrawTime );
	Get1Bool( ShowImageCount,		pdShowImageCount );
	
	Get1Bool( SaveTextLog,			pdSaveTextLog );
	Get1Bool( NewLogEveryJoin,		pdNewLogEveryJoin );
	Get1Bool( NoMovieTextLogs,		pdNoMovieTextLogs );
	
	Get1String( HostAddr,			pdHostAddr );
	Get1String( AltHostAddr,		pdProxyAddr );
	Get1Int( ClientPort,			pdClientPort );
	Get1Bool( UseSpecificPort,		pdUseArbitrary );
	
	GetTextStylesFromPrefs();
	
	Get1Int( MaxNightLevel,			pdMaxNightPct );
	Get1Int( BubbleOpacity,			pdBubbleBlitter );
	Get1Int( FriendBubbleOpacity,	pdFriendBubbleBlitter );
	Get1Int( HBColorStyle,			pdHBColorStyle );
	Get1Int( HBPlacement,			pdHBPlacement );
	
	Get1Bool( ThinkNotify,			pdThinkNotify );
	Get1Bool( DisableCmdQ,			pdDisableCmdQ );
	
	Get1Int( NumFrames,				pdNumFrames );
	Get1Int( LostFrames,			pdLostFrames );
	
	Get1Int( ClientVersion,			pdClientVersion );
	Get1Int( CompiledClient,		pdCompiledClient );
	
	Get1Bool( OpenGLEnable,			pdOpenGLEnable );
	Get1Bool( OpenGLEffects,		pdOpenGLEnableEffects );
	Get1Int( OpenGLRenderer,		pdOpenGLRenderer );
	
	GetInventoryShortcutKeys();
		
#undef Get1Bool
#undef Get1Int
#undef Get1String
}
#endif  // USE_CF_PREFERENCES


/*
**	MovePWsToKeychain()
**
**	migrate passwords out of the prefs file and into the Keychain
*/
void
MovePWsToKeychain()
{
	// don't bother if the password isn't currently being saved
	if ( Prefs::GetBool( pref_SaveAcctPass, false ) )
		{
		// decrypt it
		SimpleDecrypt2( kClientPrefsFName,
			gPrefsData.pdAcctPass, sizeof gPrefsData.pdAcctPass );
		
		// stash it
		KeychainAccess::Save( kPassType_Account,
			gPrefsData.pdAccount, gPrefsData.pdAcctPass );
		}
	
	// repeat for character password
	if ( gPrefsData.pdSavePassword )
		{
		SimpleDecrypt2( kClientPrefsFName,
			gPrefsData.pdCharPass, sizeof gPrefsData.pdCharPass );
		
		KeychainAccess::Save( kPassType_Character,
			gPrefsData.pdCharName, gPrefsData.pdCharPass );
		}
	
	// the password fields in the prefs file will be zeroed out in SavePrefsData()
}


/*
**	GetPrefsData()
**
**	Get the prefs data from the prefs file.
**	Create and init it if necessary.
*/
void
GetPrefsData()
{
	DTSError result = noErr;
	
#if USE_CF_PREFERENCES
	// this will force a rebuild unless LoadPrefsAux() gets good data later
	gPrefsData.pdVersion = -1;
	
	LoadPrefsAux();
	
#else
	
	// read the prefs record from the file
//	if ( noErr == result )
		{
		size_t prefsize;
		result = gClientPrefsFile.GetSize( kTypePreferences, 0, &prefsize );
		
		// make sure we don't overwrite anything in memory if the
		// file's record is larger than we expected it to be
		if ( noErr == result
		&&	 prefsize <= sizeof gPrefsData )
			{
			// it fits; read it in
			result = gClientPrefsFile.Read( kTypePreferences, 0, &gPrefsData, sizeof gPrefsData );
#if DTS_LITTLE_ENDIAN
			if ( noErr == result )
				PrefsSwapEndian();
#endif
			}
		else
			{
			// it's too big; ignore it and build a fresh copy
			gPrefsData.pdVersion = -1;
			}
		}
	
	// if the data is hopelessly out-of-date, throw out the entire *file*
	// and build a brand new copy - I think this may have been to avoid
	// problems with inventory items sub-prefs records...
	if ( noErr == result )
		{
		if ( gPrefsData.pdVersion < 49 )
			{
			gClientPrefsFile.Close();
			DTSFileSpec spec;
			spec.SetFileName( kClientPrefsFName );
			spec.Delete();
			
			result = gClientPrefsFile.Open( kClientPrefsFName,
							kKeyReadWritePerm | kKeyCreateFile, rPrefsFREF );
			}
		}
	
	// force rebuild in case anything bad happened
	if ( result != noErr )
		gPrefsData.pdVersion = -1;
#endif  // USE_CF_PREFERENCES
	
	// try to upgrade non-current prefs to latest
	if ( gPrefsData.pdVersion != kPrefsVersion )
		result = UpdatePrefs();
	
	// ditto for inventory items
	if ( gPrefsData.pdItemPrefsVers != kItemPrefsVersion )
		result = UpdateItemPrefs();
	
#if 0	// don't need this anymore
	// v870 DNS change: s/clanlord/deltatao/
	if ( noErr == result
	&&   0 == strcmp( gPrefsData.pdHostAddr, "server.clanlord.com:5010" ) )
		{
		StringCopySafe( gPrefsData.pdHostAddr,
			kDefaultHostAddrInternet, sizeof gPrefsData.pdHostAddr );
		}
#endif  // 0
	
	// after prefs are fully loaded, check them for sanity
	
	// step on the saved client version number
	// if it was not set by this client
	// increase the saved client version number
	// if it somehow got too low
	// (i think this would be a paranoid case)
	if ( gPrefsData.pdCompiledClient != kFullVersionNumber
	||   gPrefsData.pdClientVersion < kFullVersionNumber )
		{
		gPrefsData.pdClientVersion  = kFullVersionNumber;
		gPrefsData.pdCompiledClient = kFullVersionNumber;
		}
	
	// make sure users don't muck up the font & sizes
	if ( noErr == result )
		{
		for ( uint nn = kMsgDefault; nn < kMsgMaxStyles; ++nn )
			{
			CLStyleRecord& sr = gPrefsData.pdStyles[ nn ];
			
			switch ( nn )
				{
				case kMsgSpeech:
				case kMsgFriendSpeech:
				case kMsgMySpeech:
				case kMsgThoughtMsg:
					sr.size = 12;
					break;
				
				case kMsgBubble:
				case kMsgFriendBubble:
					sr.size = 9;
					break;
				
				default:
					// make sure the undefined messages are set to the default style
					if ( nn >= kMsgLastMsg )
						sr = gPrefsData.pdStyles[ kMsgDefault ];
					else
						sr.size = 10;
					break;
				}
			
			// force default font for all styles
			sr.font = 1; // applFont;
			
			// until v987+ we failed to byte-swap the default styles in the 'TxSt' resources.
			// The style-sanitizing routine cleaned that up, silently, but not 100% completely:
			// the 'face' fields were left unswapped. This masks off any erroneous bits:
			sr.face &= 0x07F; // allow only 'normal', 'bold', 'italic', ..., 'extend'
			}
		
		// ... except the mono-spaced variation
		gPrefsData.pdStyles[ kMsgMonoStyle ].font = 4; // kFontIDMonaco;
		
		// experiment with multiple fonts & sizes...
#if defined( DEBUG_VERSION ) && WACKY_RER_OPTIONS
		gPrefsData.pdStyles[ kMsgTimeStamp ].size = 9;
		gPrefsData.pdStyles[ kMsgTimeStamp ].font = 20; // kFontIDTimes;
#endif	// defined( DEBUG_VERSION ) && WACKY_RER_OPTIONS
		}
	
#ifdef MULTILINGUAL
	gRealLangID = gPrefsData.pdLanguageID;
#endif
	
	// decrypt the saved passwords
#if 0	// ... or rather, don't, now that they're in the Keychain
	SimpleDecrypt2( kClientPrefsFName, gPrefsData.pdAcctPass, sizeof gPrefsData.pdAcctPass );
	SimpleDecrypt2( kClientPrefsFName, gPrefsData.pdCharPass, sizeof gPrefsData.pdCharPass );
#endif  // 0
}


// Don't need endian helpers if not using prefs keyfile
#if ! USE_CF_PREFERENCES

#if DTS_LITTLE_ENDIAN
	// helper util
# define PSE(x) (x) = SwapEndian( x )

static void
SwapEndian( DTSRect& r )
{
	PSE(  r.rectLeft  );
	PSE(  r.rectTop  );
	PSE(  r.rectRight  );
	PSE(  r.rectBottom  );
}

static void
SwapEndian( DTSPoint& pt )
{
	PSE(  pt.ptH  );
	PSE(  pt.ptV  );
}

static void
SwapEndian( DTSColor& c )
{
	PSE( c.rgbRed   );
	PSE( c.rgbGreen );
	PSE( c.rgbBlue  );
}

static void
SwapEndian( CLStyleRecord& s )
{
	PSE( s.font );
	PSE( s.face );
	PSE( s.size );
	SwapEndian( s.color );
}


/*
**	PrefsSwapEndian()
**
**	byte-swap the entire gPrefsData struct
*/
void
PrefsSwapEndian()
{
	// not using BigToNativeEndian/NativeToBigEndian,
	// cuz they do exactly the same thing, but two different
	// names suggest they are somehow different;
	// i intend to use this one function to do both, so want
	// minimize confusion
	//		-- except that B2N/N2B become no-ops on BE platforms...
	PSE( gPrefsData.pdVersion );
	
	SwapEndian( gPrefsData.pdGWPos );
	SwapEndian( gPrefsData.pdTextPos );
	SwapEndian( gPrefsData.pdInvenPos );
	SwapEndian( gPrefsData.pdPlayersPos );
	SwapEndian( gPrefsData.pdMsgPos );
	
	PSE( gPrefsData.pdShowNames );
	PSE( gPrefsData.pdGodHideMonsterNames );
	PSE( gPrefsData.pdSound );
	PSE( gPrefsData.pdTimeStamp );
	PSE( gPrefsData.pdLargeWindow );
	PSE( gPrefsData.pdBrightColors );
	PSE( gPrefsData.pdMoveControls );
	PSE( gPrefsData.pdSuppressClanning );
	PSE( gPrefsData.pdSuppressFallen );
	PSE( gPrefsData.pdMusicPlay );
	PSE( gPrefsData.pdMusicVolume );
	
	PSE( gPrefsData.pdSavePassword );
	// skip strings
	PSE( gPrefsData.pdShowFrameTime );
	PSE( gPrefsData.pdSaveMsgLog );
	PSE( gPrefsData.pdSaveTextLog );
	PSE( gPrefsData.pdShowDrawTime );
	PSE( gPrefsData.pdShowImageCount );
	PSE( gPrefsData.pdFastBlitters );
	// skip more strings
	
	for ( int i = 0; i < kMsgMaxStyles; ++i )
		SwapEndian( gPrefsData.pdStyles[i] );
	
	PSE( gPrefsData.pdMaxNightPct );
	PSE( gPrefsData.pdBubbleBlitter );
	PSE( gPrefsData.pdFriendBubbleBlitter );
	PSE( gPrefsData.pdItemPrefsVers );
	PSE( gPrefsData.pdHBColorStyle );
	PSE( gPrefsData.pdHBPlacement );
	PSE( gPrefsData.pdThinkNotify );
	PSE( gPrefsData.pdDisableCmdQ );
	PSE( gPrefsData.pdNoSuppressFriends );
	PSE( gPrefsData.pdUseArbitrary );
	PSE( gPrefsData.pdClientPort );
	PSE( gPrefsData.pdTreatSharesAsFriends );
	PSE( gPrefsData.pdSoundVolume );
	PSE( gPrefsData.pdSurveys );
	PSE( gPrefsData.pdNumFrames );
	PSE( gPrefsData.pdLostFrames );
	PSE( gPrefsData.pdClientVersion );
	PSE( gPrefsData.pdCompiledClient );
	PSE( gPrefsData.pdOpenGLEnable );
	PSE( gPrefsData.pdOpenGLEnableEffects );
	PSE( gPrefsData.pdOpenGLRenderer );
	PSE( gPrefsData.pdNewLogEveryJoin );
	PSE( gPrefsData.pdNoMovieTextLogs );
	PSE( gPrefsData.pdBardVolume );
	PSE( gPrefsData.pdLanguageID );
}

#undef PSE

#endif	// DTS_LITTLE_ENDIAN
#endif  // ! USE_CF_PREFERENCES


#if USE_CF_PREFERENCES

/*
**	SavePrefsAux()
**
**	record the gPrefs data into CFPreferences
*/
static void
SavePrefsAux()
{
#define Set1Pref( key, field )	Prefs::Set( pref_ ## key, gPrefsData. field )
#define Set1Bool( key, field )	Prefs::Set( pref_ ## key, bool( gPrefsData.field ) )

	Set1Pref( GameWinPos, pdGWPos );
	Set1Pref( TextWinPos, pdTextPos );
	Set1Pref( InvenWinPos, pdInvenPos );
	Set1Pref( PlayersWinPos, pdPlayersPos );
	
	Set1Bool( Sounds,		pdSound );
	Set1Pref( SoundVolume,	pdSoundVolume );
	Set1Pref( BardVolume,	pdBardVolume );
	Set1Bool( ShowNames,	pdShowNames );
	Set1Bool( GMHideMonsterNames,	pdGodHideMonsterNames );
	Set1Bool( TimeStamp,		pdTimeStamp );
	Set1Bool( BrightColors,		pdBrightColors );
	Set1Pref( MoveControls,		pdMoveControls );
	Set1Bool( SuppressClanning,	pdSuppressClanning );
	Set1Bool( SuppressFallen,	pdSuppressFallen );
	Set1Bool( NoSuppressFriends,	pdNoSuppressFriends );
	Set1Bool( TreatSharesAsFriends, pdTreatSharesAsFriends );
	
	Set1Bool( SaveCharPass,		pdSavePassword );
	Set1Bool( SaveAcctPass,		pdSavePassword );	// FIXME
	Set1Pref( AcctName,			pdAccount );
	Set1Pref( CharName,			pdCharName );
#if 0  // these are in keychain now
	if ( gPrefsData.pdSavePassword )				// FIXME
		Set1Pref( AcctPass, pdAcctPass );
	else
		Prefs::Set( pref_AcctPass, nullptr );			// remove it
	if ( gPrefsData.pdSavePassword )
		Set1Pref( CharPass, pdCharPass );
	else
		Prefs::Set( pref_CharPass, nullptr );			// remove this too
#endif
	
	Set1Bool( ShowFrameTime, pdShowFrameTime );
	Set1Bool( SaveMsgLog, pdSaveMsgLog );
	Set1Bool( ShowDrawTime, pdShowDrawTime );
	Set1Bool( ShowImageCount, pdShowImageCount );
	
	Set1Bool( SaveTextLog, pdSaveTextLog );
	Set1Bool( NewLogEveryJoin, pdNewLogEveryJoin );
	Set1Bool( NoMovieTextLogs, pdNoMovieTextLogs );
	
	Set1Pref( HostAddr, pdHostAddr );
	Set1Pref( AltHostAddr, pdProxyAddr );
	Set1Pref( ClientPort, pdClientPort );
	Set1Bool( UseSpecificPort, pdUseArbitrary );
	
	SaveTextStylesToPrefs();
	
	Set1Pref( MaxNightLevel, pdMaxNightPct );
	Set1Pref( BubbleOpacity, pdBubbleBlitter );
	Set1Pref( FriendBubbleOpacity, pdFriendBubbleBlitter );
	Set1Pref( HBColorStyle, pdHBColorStyle );
	Set1Pref( HBPlacement, pdHBPlacement );
	
	Set1Bool( ThinkNotify, pdThinkNotify );
	Set1Bool( DisableCmdQ, pdDisableCmdQ );
	
	Set1Pref( NumFrames, pdNumFrames );
	Set1Pref( LostFrames, pdLostFrames );
	
	Set1Pref( ClientVersion, pdClientVersion );
	Set1Pref( CompiledClient, pdCompiledClient );
	
	Set1Bool( OpenGLEnable, pdOpenGLEnable );
	Set1Bool( OpenGLEffects, pdOpenGLEnableEffects );
	Set1Pref( OpenGLRenderer, pdOpenGLRenderer );
	
	SaveInventoryShortcutKeys();
		
#undef Set1Bool
#undef Set1Pref

}
#endif  // USE_CF_PREFERENCES


/*
**	SavePrefsData()
**
**	save the prefs data to the prefs file (...or CFPrefs)
*/
void
SavePrefsData()
{
	GetWindowPosition( gGameWindow,  &gPrefsData.pdGWPos    );
	GetWindowPosition( gTextWindow,  &gPrefsData.pdTextPos  );
	GetWindowPosition( gInvenWindow, &gPrefsData.pdInvenPos );
	GetWindowPosition( gPlayersWindow, &gPrefsData.pdPlayersPos );
	GetMsgWinPos( &gPrefsData.pdMsgPos );
	
	// zap the saved passwords to zeros if the player doesn't want to save them
	// ... in fact, always zap them now that they're in the Keychain
	if ( true /* not gPrefsData.pdSavePassword */ )
		{
		memset( gPrefsData.pdAcctPass, 0, sizeof gPrefsData.pdAcctPass );
		memset( gPrefsData.pdCharPass, 0, sizeof gPrefsData.pdCharPass );
		}
	
	// encrypt the saved passwords (even if they got zapped; since they'll be
	// decrypted on startup regardless)
	// ... but don't even bother with this once they are migrated to keychain
#if 0
	SimpleEncrypt2( kClientPrefsFName, gPrefsData.pdAcctPass, sizeof gPrefsData.pdAcctPass );
	SimpleEncrypt2( kClientPrefsFName, gPrefsData.pdCharPass, sizeof gPrefsData.pdCharPass );
#endif  // 0
	
#ifdef MULTILINGUAL
	gPrefsData.pdLanguageID = gRealLangID;
#endif
	
#if USE_CF_PREFERENCES
	SavePrefsAux();
#else
	// write the prefs to the database file
	PrefsData savePrefsData = gPrefsData;
# if DTS_LITTLE_ENDIAN
	PrefsSwapEndian();
# endif
	(void) gClientPrefsFile.Write( kTypePreferences, 0, &gPrefsData, sizeof gPrefsData );
	gPrefsData = savePrefsData;
#endif  // USE_CF_PREFERENCES
	
	// ensure CFPreferences are saved as well
	(void) CFPreferencesAppSynchronize( kCFPreferencesCurrentApplication );
}

#pragma mark -


/*
**	DoCharacterManager()
**
**	put up the character manager dialogs
*/
void
DoCharacterManager()
{
	// check the semaphore
	if ( gPlayingGame )
		return;
	
	CharacterManager();
}


/*
**	DidChooseMovieFile()
**
**	callback from NavSvcs; invoked after the user has chosen a movie file
*/
OSStatus
DidChooseMovieFile( CFURLRef url )
{
	DTSFileSpec moviespec;
	OSStatus err = DTSFileSpec_SetFromURL( &moviespec, url );
	if ( noErr == err )
		{
		err = CCLMovie::StartReadMovie( &moviespec );
		if ( noErr == err )
			{
			char msg[ 256 ];
			snprintf( msg, sizeof msg, _(TXTCL_STARTING_MOVIE), moviespec.GetFileName() );
			ShowInfoText( msg );
			
			PlayGame();		// really means "PlayMovie()", here
			
			snprintf( msg, sizeof msg, _(TXTCL_END_OF_MOVIE), moviespec.GetFileName() );
			ShowInfoText( msg );
			}
		else
			{
			const char * errMsg;
			switch ( err )
				{
				case CCLMovie::kMovieTooOldErr:
					errMsg = _(TXTCL_MOVIE_TO_OLD);
					break;
				case CCLMovie::kMovieTooNewErr:
					errMsg = _(TXTCL_MOVIE_TO_NEW);
					break;
				default:
					errMsg = _(TXTCL_MOVIE_NOT_READABLE);
				}
			ShowMsgDlg( errMsg );
			}
		}
	else
		{
#ifdef DEBUG_VERSION
		GenericError( "Unable to open the movie. (code=%d)", int( err ) );
#else
		Beep();
#endif  // DEBUG_VERSION
		}
	
	return err;
}


/*
**	DoSelectMovieFile()
**
**	put up a dialog to choose a movie file
*/
void
DoSelectMovieFile()
{
	NavDialogCreationOptions opts;
	OSStatus err = NavGetDefaultDialogCreationOptions( &opts );
	__Check_noErr( err );
	if ( noErr == err )
		{
		opts.optionFlags &= ~ (kNavAllowMultipleFiles | kNavAllowPreviews);
//		opts.optionFlags |= kNavNoTypePopup;
		
		opts.clientName = static_cast<CFStringRef> (
			CFBundleGetValueForInfoDictionaryKey( CFBundleGetMainBundle(), kCFBundleNameKey ) );
		
		opts.windowTitle = CFSTR("Choose Movie");
		opts.message = CFSTR(_(TXTCL_SELECT_MOVIE_FILE));
		opts.modality = kWindowModalityWindowModal;
		opts.parentWindow = DTS2Mac(gGameWindow);
		}
	if ( noErr == err )
		{
		CLApp * app = static_cast<CLApp *>( gDTSApp );
		err = AskChooseFile( *app, opts );
		}
}


/*
**	DoSelect()
**
**	collect character name and password
**	and join the game
*/
void
DoSelect()
{
	// check the semaphores
	if ( gPlayingGame )
		return;
	
	if ( gChecksumMsgGiven )
		{
		ShowChecksumError( 1 );
		return;
		}
	
	DTSError result = SelectCharacter();
	if ( noErr == result )
		PlayGame();
}


/*
**	DoJoin()
**
**	make sure we have a character name
**	and join the game
*/
void
DoJoin()
{
	if ( gChecksumMsgGiven )
		{
		ShowChecksumError( 1 );
		return;
		}

	if ( gPrefsData.pdCharName[0] == '\0' )
		DoSelect();
	else
	// require a password unless it's a demo, or there's a saved one
	if ( strncasecmp( gPrefsData.pdCharName, "Agratis ", 8 ) != 0
	&&	 not gPrefsData.pdSavePassword )
		{
		DTSError result = EnterPassword();
		if ( noErr == result )
			PlayGame();
		}
	else
		PlayGame();
}


/*
**	PlayGame()
**
**	play the game
*/
void
PlayGame()
{
	// OK, problem: on X, if a player chooses "Select Char" or the char manager,
	// the app goes into a subsidiary event loop here, and doesn't return to the
	// carbon event manager until they stop playing. Which means the CEM,
	// in turn, won't be able to dehilite the chosen menu item's title until
	// way too late. So we have to unhilite the menu ourself.
	::HiliteMenu( 0 );
	
	// check and set the semaphore
	if ( gPlayingGame )
		return;
	gPlayingGame = true;

	// check some commonly modified images
	DTSError result = ChecksumUsualSuspects();
	if ( noErr != result )
		return;
	
	SetPlayMenuItems();
	
	// we now need somewhat more CPU
	int oldSleep = gDTSApp->SetSleepTime( 6 );
	
	// clear the command string to be sent
	ClearSendString( gCommandNumber );
	
	InitGameState();
	
	// clear the click toggle state
	gClickState = 0;
	
	// initialize the sound
	CLInitSound();
	
	InitPlayers();
	
	// read saved friends
	ReadFriends( gPrefsData.pdCharName );
	
	// start a new text log if we do that every join
	// or if they changed characters or started a movie
	if ( gPrefsData.pdNewLogEveryJoin
	||	strcasecmp( gLogCharName, gPrefsData.pdCharName ) != 0
	||	CCLMovie::IsReading() )
		{
		StartTextLog( );
		}
	
	if ( CCLMovie::IsReading() ) 	// Initialize the client for movie
		result = InitReadMovie();
	else
		{
		// initialize the communications
		result = InitComm();
		
#if USE_MACRO_FOLDER
		// initialize macros
		if ( noErr == result )
			InitMacros();
#endif	// USE_MACRO_FOLDER
		
		// set cmd-q menu status by prefs
		// if the prefs is true then the status must be set false
		SetCommandQStatus( not gPrefsData.pdDisableCmdQ );
		}
	
#ifdef MULTILINGUAL
	// send our client language to the server
	char langidstr[ 8 ];  	// hold the string representation of the language id
	snprintf( langidstr, sizeof langidstr, "%d", gRealLangID );
	SendCommand( "be-lang", langidstr, true );
# ifdef DEBUG_VERSION
//	ShowMessage( "Calling be-lang: %s.", langidstr );
# endif // DEBUG_VERSION
#endif // MULTILINGUAL
	
	// play the game
	if ( noErr == result )
		{
		gDoneFlag = false;
		
		// this is the "inner main event loop" I was talking about
		while ( not gDoneFlag )
			{
			if ( not gPlayingGame )
				break;
			
#if ENABLE_MUSIC_FILES
			if ( not gMusicDisabled )
				RunCLMusic();
#endif
			
			if ( gInBack )
				{
				RunApp();
				DoComm();
				}
			else
				{
				gDoComm = true;
				RunApp();
				gDoComm = false;
				}
			}
		}

	// restore cmd-q function
	SetCommandQStatus( true );
	
	// inform the user
	// just in case they can't guess
	if ( CCLMovie::IsReading() ) // BMH
		ShowInfoText( _(TXTCL_END_OF_MOVIE_FILE) );
	else
		ShowInfoText( _(TXTCL_NO_LONGER_CONNECTED) );
	
	// redraw one final time, to drive the point home
	RedrawCLWindow();
	
	// close the communications
	if ( not CCLMovie::IsReading() )
		ExitComm();
	
	// close the sound channels
	CLExitSound();
	
	// write the saved friends
	WriteFriends( gPrefsData.pdCharName );
	
	// Reset the players list
	DisposePlayers();
	
	// reset the inventory list
	ResetInventory();
	
#if USE_MACRO_FOLDER
	// Kill loaded macros
	KillMacros();
#endif
	
	// free the semaphore
	gPlayingGame = false;
	
	// we don't need the CPU anymore
	gDTSApp->SetSleepTime( oldSleep );
	
	// stop the movie unless we're recording to it
	if ( not CCLMovie::IsRecording() )
		CCLMovie::StopMovie();
	
	SetPlayMenuItems();
	
	// X the dock icon, if in background
	BadgeDockIcon();
}


/*
**	BadgeDockIcon()
**
**	Mark the dock icon to show that we've been disconnected
*/
void
BadgeDockIcon()
{
	if ( not gInBack )
		return;
	
	// Are we using the older call (BeginCGContextForApplicationDockTile()),
	// or the newer one (HIApplicationCreateDockTileContext())?
	// Thing is, what the docs don't tell you is that the new call fails to pre-draw
	// the app's own icon in the CGContext; it starts out totally blank. But the older
	// API _does_ show the app icon automatically.
	// Since I don't want to write the code to load the icon file and stamp it
	// into the context, let's just stick with the older API for now.
	
#define USE_NEW_DOCK_TILE_API		0

	HISize sz;
	
#if USE_NEW_DOCK_TILE_API
	// context dimensions are not known in advance
	CGContextRef cg = HIApplicationCreateDockTileContext( &sz );
#else
	// The context's dimensions are fixed by the OS at 128x128.
	sz = CGSizeMake( 128, 128 );
	CGContextRef cg = BeginCGContextForApplicationDockTile();
#endif
	if ( cg )
		{
		// this was originally written for BeginCGContextForAppDockTile() and its
		// fixed-size 128x128 tile; so we can multiply all the old coords by this
		// scale factor and have everything work out.
		CGFloat scale = sz.width / 128;
		
		CGContextSetRGBStrokeColor( cg, 1, 0, 0, 1 );	// opaque 100% red
		CGContextSetLineWidth( cg, 6 * scale );
		
#if 0	// draw an 'X' -- n.b. NOT updated to use 'scale' factor
		
		// geography of the X
		enum {
			Xsize = 45,
			Xleft = 75,
			Xtop = 10
			};
		
		CGContextSetLineCap( cg, kCGLineCapRound );
		CGContextMoveToPoint( cg, Xleft, 128 - Xtop );
		CGContextAddLineToPoint( cg, Xleft + Xsize, 128 - (Xtop + Xsize) );
		CGContextMoveToPoint( cg, Xleft, 128 - (Xtop + Xsize) );
		CGContextAddLineToPoint( cg, Xleft + Xsize, 128 - Xtop );
		CGContextStrokePath( cg );
		
#else	// draw a "not" symbol in the upper-right quadrant of the tile
		
		enum {
			Orig = 80,
			Diameter = 120 - Orig,
			
			Radius = Diameter / 2,
			Ctr = Orig + Radius,
			Corner = (1414 * Radius) / 2000,	// sqrt(2) / 2
			};
		
		const CGFloat O2 = Ctr * scale,
				R2 = Radius * scale,
				D2 = Diameter * scale,
				C2 = Corner * scale;
		
		CGContextTranslateCTM( cg, O2, O2 );
		CGContextAddEllipseInRect( cg, CGRectMake( -R2, -R2, D2, D2 ) );
		CGContextMoveToPoint( cg, - C2, C2 );
		CGContextAddLineToPoint( cg, C2, -C2 );
		CGContextStrokePath( cg );
		CGContextTranslateCTM( cg, -O2, -O2 );
		
#endif	// 0
		
		CGContextFlush( cg );
		EndCGContextForApplicationDockTile( cg );
		}
}


/*
**	ResetDockIcon()
**
**	Reset the dock icon
*/
void
ResetDockIcon()
{
	::RestoreApplicationDockTileImage();
}


/*
**	SetPlayMenuItems()
**
**	enable and disable the items affected by whether we're playing or not
*/
void
SetPlayMenuItems()
{
	if ( gConnectGame )
		{
		gDTSMenu->SetFlags( mCharManager, kMenuDisabled );
		gDTSMenu->SetFlags( mSelectChar,  kMenuDisabled );
		gDTSMenu->SetFlags( mJoinGame,    kMenuDisabled );
		gDTSMenu->SetFlags( mDisconnect,  kMenuDisabled );
		gDTSMenu->SetFlags( mViewMovie,   kMenuDisabled );
		gDTSMenu->SetFlags( mRecordMovie, kMenuDisabled );
		
#if USE_MACRO_FOLDER
		gDTSMenu->SetFlags( mReloadMacros, kMenuDisabled );
#endif
		}
	else
		{
		uint flagEnabledPlaying;
		uint flagDisabledPlaying;
		
		if ( gPlayingGame )
			{
			flagEnabledPlaying  = kMenuNormal;
			flagDisabledPlaying = kMenuDisabled;
			}
		else
			{
			flagEnabledPlaying  = kMenuDisabled;
			flagDisabledPlaying = kMenuNormal;
			}
		
		if ( CCLMovie::HasMovie() )
			{
			if ( CCLMovie::IsRecording() )
				gDTSMenu->SetFlags( mRecordMovie, kMenuNormal | kMenuChecked );
			else
				gDTSMenu->SetFlags( mRecordMovie, kMenuDisabled );
			}
		else
			gDTSMenu->SetFlags( mRecordMovie, kMenuNormal );
		
		gDTSMenu->SetFlags( mCharManager, flagDisabledPlaying );
		gDTSMenu->SetFlags( mSelectChar,  flagDisabledPlaying );
		gDTSMenu->SetFlags( mJoinGame,    flagDisabledPlaying );
		gDTSMenu->SetFlags( mDisconnect,  flagEnabledPlaying  );
		gDTSMenu->SetFlags( mViewMovie,   flagDisabledPlaying );
		
#if USE_MACRO_FOLDER
		gDTSMenu->SetFlags( mReloadMacros, flagEnabledPlaying );
#endif
		}
}


/*
**	SelectShowWindow()
**
**	show and select a window
*/
void
SelectShowWindow( DTSWindow * window )
{
	if ( window )
		{
		window->Show();
		
		// un-minimize if necessary
		if ( window->IsMinimized() )
			window->SetMinimized( false );
		
		window->GoToFront();
		}
}


/*
**	AllocDescTable()
**
**	allocate the descriptor table
*/
void
AllocDescTable()
{
	DescTable * table = NEW_TAG("DescTable") DescTable[ kDescTableSize ];
	CheckPointer( table );
	
	gDescTable = table;
}


/*
**	GetWindowPosition()
**
**	return the position of the window
*/
void
GetWindowPosition( const DTSWindow * window, DTSRect * pos )
{
	if ( not window )
		return;
	
	DTSPoint pt;
	window->GetPosition( &pt );
	
	pos->Move( pt.ptH, pt.ptV );
	DTSRect viewBounds;
	window->GetBounds( &viewBounds );
	pos->Size(
		viewBounds.rectRight  - viewBounds.rectLeft,
		viewBounds.rectBottom - viewBounds.rectTop );
}


/*
**	GetMainPrefs()
**
**	display and run the preferences dialog
*/
void
GetMainPrefs()
{
	MainPrefsDlg dlg;
	dlg.mcdVolume = gPrefsData.pdSoundVolume;
	dlg.mcdBardVolume = gPrefsData.pdBardVolume;
	
	dlg.Run();
	
	// restore the sound volume unless he hit OK
	if ( kHICommandOK != dlg.Dismissal() )
		{
		gPrefsData.pdSoundVolume = dlg.mcdVolume;
		SetSoundVolume( gPrefsData.pdSoundVolume );
		}
}


/*
**	MainPrefsDlg::Init()
*/
void
MainPrefsDlg::Init()
{
	SetValue( Item(mcdClickRadioGroup), gPrefsData.pdMoveControls - kMoveClickHold + 1 );
	SetValue( Item(mcdClanningCheck), gPrefsData.pdSuppressClanning ? 1 : 0 );
	SetValue( Item(mcdFallenCheck), gPrefsData.pdSuppressFallen ? 1 : 0 );
	SetValue( Item(mcdShowNamesCheck), gPrefsData.pdShowNames ? 1 : 0 );
	
#ifdef AUTO_HIDENAME
	// if Auto_HideNames, the pref takes on a new & different meaning.
	// change the control to match.
	SetText( Item(mcdShowNamesCheck), _(TXTCL_AUTO_HIDE_NAMES) );
#endif	// AUTO_HIDENAME
	
	SetValue( Item(mcdTimeStampsCheck), gPrefsData.pdTimeStamp ? 1 : 0 );
	SetValue( Item(mcdBrightColorsCheck), gPrefsData.pdBrightColors ? 1 : 0 );

	// map night percentages to menu item indices
	int nightNum;
	switch ( gPrefsData.pdMaxNightPct )
		{
		case 0:		nightNum = 1;	break;
		case 25:	nightNum = 2;	break;
		case 50:	nightNum = 3;	break;
		default:
		case 75:	nightNum = 4;	break;
		case 100:	nightNum = 5;	break;
		}
	SetValue( Item(mcdNightPopupMenu), nightNum );
	
	SetValue( Item(mcdNoSuppressFriendsCheck), gPrefsData.pdNoSuppressFriends ? 1 : 0 );
	
	// deal with sounds
	SetValue( Item(mcdVolumeSlider), mcdVolume );
	SetValue( Item(mcdBardVolumeSlider), mcdBardVolume );
	AbleItem( Item(mcdVolumeSlider), gPrefsData.pdSound );
	AbleItem( Item(mcdBardVolumeSlider), gPrefsData.pdSound );
	
	AdjustFriends();
	ShowText();
}


/*
**	MainPrefsDlg::AdjustFriends()
**
**	enable/disable NoSuppressFriendsCheck accordingly
*/
void
MainPrefsDlg::AdjustFriends() const
{
	bool noClanning = GetValue( Item( mcdClanningCheck ) );
	bool noFallen = GetValue( Item( mcdFallenCheck ) );
	AbleItem( Item(mcdNoSuppressFriendsCheck), noClanning || noFallen );
}


/*
**	MainPrefsDlg::DoCommand()
**
**	handle commands from dialog UI
*/
bool
MainPrefsDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = true;
	
	switch ( cmd.commandID )
		{
		case kHICommandOK:
			if ( Validate() )
				{
				SaveInfo();
				Dismiss( cmd.commandID );
				}
			break;
		
		case cmd_ClickRadio:
			ShowText();
			break;
		
		case cmd_SuppressMsgs:
			AdjustFriends();
			break;
		
		case cmd_AdvancedPrefs:
			SetAdvancedPrefs();
			break;
		
		case cmd_ServerAddress:
			GetHostAddr();
			break;
		
		case cmd_SetVolume:
			gPrefsData.pdSoundVolume = GetValue( Item( mcdVolumeSlider ) );
			SetSoundVolume( gPrefsData.pdSoundVolume );
			break;
		
		case cmd_SetBardVolume:
			SetBardVolume( GetValue( Item( mcdBardVolumeSlider ) ) );
			break;
		
		default:
			result = false;
		}
	
	return result;
}


/*
**	MainPrefsDlg::SaveInfo()
*/
void
MainPrefsDlg::SaveInfo()
{
	gPrefsData.pdMoveControls		= GetValue( Item( mcdClickRadioGroup ) ) - 1 + kMoveClickHold;
	
	gPrefsData.pdSuppressClanning	= GetValue( Item( mcdClanningCheck) );
	gPrefsData.pdSuppressFallen		= GetValue( Item( mcdFallenCheck ) );
	gPrefsData.pdShowNames			= GetValue( Item( mcdShowNamesCheck) );
	gPrefsData.pdLargeWindow		= true;	// mcdLargeWindow;
	gPrefsData.pdTimeStamp			= GetValue( Item( mcdTimeStampsCheck) );
	gPrefsData.pdBrightColors		= GetValue( Item( mcdBrightColorsCheck ) );
	int nightVal = 0;
	switch ( GetValue( Item( mcdNightPopupMenu ) ) )
		{
		default:
		case 1:	nightVal = 0;  break;
		case 2: nightVal = 25; break;
		case 3: nightVal = 50; break;
		case 4: nightVal = 75; break;
		case 5: nightVal = 100; break;
		}
	gPrefsData.pdMaxNightPct		= nightVal;
	gPrefsData.pdNoSuppressFriends	= GetValue( Item( mcdNoSuppressFriendsCheck ) );
	
	gClickState = 0;
	
	gPrefsData.pdSoundVolume = GetValue( Item(mcdVolumeSlider) );
	gPrefsData.pdBardVolume  = GetValue( Item(mcdBardVolumeSlider) );
	
	SetBackColors();
}


/*
**	MainPrefsDlg::ShowText()
*/
void
MainPrefsDlg::ShowText() const
{
	const char * text;
	if ( 1 == GetValue( Item( mcdClickRadioGroup) ) )
		text = _(TXTCL_RADIO_CLICKHOLD);
	else
		text = _(TXTCL_RADIO_CLICKTOGGLES);
	
	SetText( Item(mcdExplanationText), text );
}


#ifdef DEBUG_VERSION

/*
**	VersionDlg::Init()
*/
void
VersionDlg::Init()
{
	vdClient = gPrefsData.pdClientVersion;
	vdImages = gImagesVersion;
	vdSounds = gSoundsVersion;
	SetVNs();
}


/*
**	VersionDlg::SetVNs()
*/
void
VersionDlg::SetVNs() const
{
	VNtoText( vdClient, vdClientEdit );
	VNtoText( vdImages, vdImagesEdit );
	VNtoText( vdSounds, vdSoundsEdit );
}


/*
**	VersionDlg::VNtoText()
*/
void
VersionDlg::VNtoText( int version, int itemnum ) const
{
	int major = (version >> 8) & 0x00FFFFFF;
	int minor = version & 0x0FF;
	
	char buff[ 32 ];
	if ( minor )
		snprintf( buff, sizeof buff, "%d.%d", major, minor );
	else
		snprintf( buff, sizeof buff, "%d", major );
	
	SetText( Item( itemnum ), buff );
}


/*
**	VersionDlg::DoCommand()
**
**	react to button clicks
*/
bool
VersionDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = false;
	
	if ( kHICommandRevert == cmd.commandID )
		{
		ResetVNs();
		result = true;
		}
	
	return result;
}


/*
**	VersionDlg::ResetVNs()
**
**	restore the on-disk version numbers
*/
void
VersionDlg::ResetVNs()
{
	vdClient = kFullVersionNumber;
	
	int32_t version;
	DTSError err = ReadFileVersion( &gClientImagesFile, &version );
	if ( noErr == err )
		vdImages = version;
	else
		vdImages = gImagesVersion;
	
	err = ReadFileVersion( &gClientSoundsFile, &version );
	if ( noErr == err )
		vdSounds = version;
	else
		vdSounds = gSoundsVersion;
	
	SetVNs();
	
	SelectText( Item(vdClientEdit), 0, 0x3FFF );
}


/*
**	VersionDlg::SaveInfo()
*/
void
VersionDlg::SaveInfo()
{
	TextToVN( vdClientEdit, gPrefsData.pdClientVersion );
	TextToVN( vdImagesEdit, gImagesVersion );
	TextToVN( vdSoundsEdit, gSoundsVersion );
}


/*
**	VersionDlg::TextToVN()
*/
void
VersionDlg::TextToVN( int itemnum, int32_t& version ) const
{
	char buff[ 32 ];
	GetText( Item(itemnum), buff, sizeof buff );
	
	int major = 0;
	int minor = 0;
	sscanf( buff, "%d.%d", &major, &minor );
	version = ( major << 8 ) + ( minor & 0xFF );
}


/*
**	DrawRateControlDlg
**
**	controls how the client auto-toggles between fast & slow blitters
*/
class DrawRateDlg : public HIDialog, public GameTickler
{
	enum
		{
		drdRateHiEdit = 1,
		drdRateLoEdit = 2
		};
	
	static const OSType Signature = 'SDPr';
	static HIViewID Item( int itemNo ) { return (HIViewID){ Signature, itemNo }; }
	
	virtual CFStringRef	GetNibFileName() const	{ return CFSTR("DrawRate"); }
	virtual CFStringRef	GetPositionPrefKey() const { return CFSTR("DrawRateDlgPosition"); }
	
	virtual void	Init();
	virtual bool	Validate() const;
	
public:
	void			SaveData() const;
};


/*
**	DrawRateDlg::Init()
**
**	prep dialog's UI
*/
void
DrawRateDlg::Init()
{
	HIDialog::Init();
	SetNumber( Item( drdRateHiEdit ), gFastDrawLimit );
	SetNumber( Item( drdRateLoEdit ), gSlowDrawLimit );
}


/*
**	DrawRateDlg::Validate()
**
**	ensure user's input is not absurdly out of range
*/
bool
DrawRateDlg::Validate() const
{
	int slow = GetNumber( Item( drdRateLoEdit ) );
	int fast = GetNumber( Item( drdRateHiEdit ) );
	
	if ( fast < 50 || fast > 230 )
		{
		Beep();
		SelectText( Item( drdRateHiEdit ) );
		GenericError( "Fast threshold must be between 50 and 230 msec." );
		return false;
		}
	
	if ( slow <= fast || slow > 245 )
		{
		Beep();
		SelectText( Item( drdRateLoEdit ) );
		GenericError( "Slow threshold must be between %d and 245 msec.", fast );
		return false;
		}
	
	return true;
}


/*
**	DrawRateDlg::SaveData()
**
**	apply the dialog's settings, since the user has just hit OK
*/
void
DrawRateDlg::SaveData() const
{
	gSlowDrawLimit = GetNumber( Item( drdRateLoEdit ) );
	gFastDrawLimit = GetNumber( Item( drdRateHiEdit ) );
}


/*
**	GetDrawRate()
**
**	dialog to get new fast-frame draw limit
*/
void
GetDrawRate()
{
	DrawRateDlg dlg;
	dlg.Run();
	if ( kHICommandOK == dlg.Dismissal() )
		dlg.SaveData();
}

#endif  // DEBUG_VERSION


/*
**	AdvancedPrefsDlg
**
**	stuff that's too scary for the main prefs
*/
class AdvancedPrefsDlg : public HIDialog, public GameTickler
{
	static const OSType Signature = 'APrf';
	static HIViewID Item( int n ) { return (HIViewID){ Signature, n }; }
	virtual CFStringRef GetNibFileName() const { return CFSTR("Advanced Prefs"); }
	virtual CFStringRef GetPositionPrefKey() const { return CFSTR("AdvPrefDlgPosition"); }
	
public:
	int		apdHBToggle;
	int		apdHBColorStyle;
	int		apdHBPlacement;
	bool	apdThinkNotify;
	bool	apdTreatSharesAsFriends;
	bool	apdDisableCmdQ;
	bool	apdNewLogEveryJoin;
	bool	apdNoMovieTextLogs;
#ifdef USE_OPENGL
	bool	apdOpenGLEnable;
	bool	apdForceGenericRenderer;
	bool	apdOpenGLPermitAnyRenderer;
	bool	apdOpenGLEffects;
	bool	apdOpenGLDisableShadowEffects;
#endif	// USE_OPENGL
	
private:
	virtual void	Init();
	virtual bool	DoCommand( const HICommandExtended& );
	
public:
	void			SaveInfo() const;
	
private:
	enum
		{
		apdHBCheck = 1,				// group control
		apdColorRadioGroup,
		apdPlacementRadioGroup,
#ifdef USE_OPENGL
		// these only exist in the AM dialog, because the OGL client is carbon only
		apdOpenGLEnableCheck = 4,
		apdRendererRadioGroup,
		apdOpenGLEffectsCheck,
#endif	// USE_OPENGL
		apdThinkNotifyCheck = 7,
		apdTreatSharesAsFriendsCheck,
		apdDisableCmdQCheck,
		apdNewLogEveryJoinCheck,
		apdNoMovieTextLogsCheck,
		};
	
	enum {
		cmd_ToggleHealthBars = '~HBs',
		cmd_ToggleOpenGL = '~OGL',
		cmd_SubstituteGLFX = 'GLFx',
		cmd_ChooseRenderer = 'Rndr',
		};
};


/*
**	SetAdvancedPrefs()
*/
void
SetAdvancedPrefs()
{
	AdvancedPrefsDlg dlg;
	dlg.Run();
	
	if ( kHICommandOK == dlg.Dismissal() )
		dlg.SaveInfo();
}


/*
**	AdvancedPrefsDlg::Init()
**
*/
void
AdvancedPrefsDlg::Init()
{
	apdHBToggle = not (gPrefsData.pdHBColorStyle >> 16);
	apdHBColorStyle = gPrefsData.pdHBColorStyle & 0x0FFFF;
	apdHBPlacement  = gPrefsData.pdHBPlacement;
	apdThinkNotify  = gPrefsData.pdThinkNotify;
	apdTreatSharesAsFriends = gPrefsData.pdTreatSharesAsFriends;
	apdDisableCmdQ  = gPrefsData.pdDisableCmdQ;
	apdNewLogEveryJoin = gPrefsData.pdNewLogEveryJoin;
	apdNoMovieTextLogs = gPrefsData.pdNoMovieTextLogs;

	SetValue( Item(apdColorRadioGroup ), apdHBColorStyle + 1 );
	SetValue( Item(apdPlacementRadioGroup), apdHBPlacement + 1 );
	
	SetValue( Item( apdHBCheck ), apdHBToggle ? 1 : 0 );
	SetValue( Item( apdThinkNotifyCheck ), apdThinkNotify ? 1 : 0 );
	SetValue( Item( apdTreatSharesAsFriendsCheck ), apdTreatSharesAsFriends ? 1 : 0 );
	SetValue( Item( apdDisableCmdQCheck ), apdDisableCmdQ ? 1 : 0 );
	SetValue( Item( apdNewLogEveryJoinCheck ), apdNewLogEveryJoin ? 1 : 0 );
	SetValue( Item( apdNoMovieTextLogsCheck ), apdNoMovieTextLogs ? 1 : 0 );

	AbleItem( Item( apdColorRadioGroup ), apdHBToggle );
	AbleItem( Item( apdPlacementRadioGroup ), apdHBToggle );
	
#ifdef USE_OPENGL
	if ( gOpenGLAvailable )
		{
		apdOpenGLEnable = gOpenGLEnable;
		apdForceGenericRenderer = gOpenGLForceGenericRenderer;
		apdOpenGLPermitAnyRenderer = gOpenGLPermitAnyRenderer;
		apdOpenGLEffects = gOpenGLEnableEffects;
		
		AbleItem( Item( apdOpenGLEnableCheck ), true );
		SetValue( Item( apdOpenGLEnableCheck ), apdOpenGLEnable ? 1 : 0 );
		if ( apdOpenGLEnable )
			{
			AbleItem( Item(apdRendererRadioGroup), apdOpenGLEnable );
			if ( apdForceGenericRenderer )
				SetValue( Item(apdRendererRadioGroup), 2 );
			else
			if ( apdOpenGLPermitAnyRenderer )
				SetValue( Item( apdRendererRadioGroup ), 3 );
			else
				SetValue( Item( apdRendererRadioGroup ), 1 );
			
			SetValue( Item( apdOpenGLEffectsCheck ), apdOpenGLEffects ? 1 : 0 );
			AbleItem( Item( apdOpenGLEffectsCheck ), true );
			}
		else
			{
			SetValue( Item( apdRendererRadioGroup ), 1 );
			AbleItem( Item( apdRendererRadioGroup ), false );
			SetValue( Item( apdOpenGLEffectsCheck ), 0 );
			AbleItem( Item( apdOpenGLEffectsCheck ), false );
			}
		}
	else
		{
		AbleItem( Item(apdOpenGLEnableCheck), false );
		AbleItem( Item(apdRendererRadioGroup), false );
		AbleItem( Item(apdOpenGLEffectsCheck), false );
		}
#endif	// USE_OPENGL
};


/*
**	AdvancedPrefsDlg::DoCommand()
**
**	react to button clicks
*/
bool
AdvancedPrefsDlg::DoCommand( const HICommandExtended& cmd )
{
	bool result = true;
	
	switch ( cmd.commandID )
		{
		case cmd_ToggleHealthBars:
			apdHBToggle = GetValue( Item( apdHBCheck ) );
			AbleItem( Item( apdColorRadioGroup ), apdHBToggle );
			AbleItem( Item( apdPlacementRadioGroup ), apdHBToggle );
			break;
		
#ifdef USE_OPENGL
		case cmd_ToggleOpenGL:
			apdOpenGLEnable = GetValue( Item( apdOpenGLEnableCheck) );
			AbleItem( Item( apdRendererRadioGroup ), apdOpenGLEnable );
			AbleItem( Item( apdOpenGLEffectsCheck ), apdOpenGLEnable );
			
			if ( apdOpenGLEnable )
				{
				const HIViewID renderGrp = Item( apdRendererRadioGroup );
				if ( apdForceGenericRenderer )
					SetValue( renderGrp, 2 );
				else
				if ( apdOpenGLPermitAnyRenderer )
					SetValue( renderGrp, 3 );
				else
					SetValue( renderGrp, 1 );
				
				SetValue( Item( apdOpenGLEffectsCheck ), apdOpenGLEffects );
				}
			else
				{
				SetValue( Item( apdRendererRadioGroup ), 0 );
				SetValue( Item( apdOpenGLEffectsCheck ), 0 );
				}
			break;
		
		case cmd_SubstituteGLFX:
			apdOpenGLEffects = GetValue( Item( apdOpenGLEffectsCheck ) );
			break;
		
		case cmd_ChooseRenderer:
			{
			int v = GetValue( Item( apdRendererRadioGroup ) );
			apdForceGenericRenderer = ( 2 == v );
			apdOpenGLPermitAnyRenderer = (3 == v);
			}
			break;
#endif  // USE_OPENGL
		
		default:
			result = false;
		}
	
	return result;
}


/*
**	AdvancedPrefsDlg::SaveInfo()
**
**	transfer user's new settings from GUI into gPrefs
*/
void
AdvancedPrefsDlg::SaveInfo() const
{
	int wantHB = GetValue( Item( apdHBCheck ) );
	int colorStyle = GetValue( Item( apdColorRadioGroup ) ) - 1;
	
	gPrefsData.pdHBColorStyle = MakeLong( not wantHB, colorStyle );
	gPrefsData.pdHBPlacement = GetValue( Item( apdPlacementRadioGroup ) ) - 1;
	gPrefsData.pdThinkNotify = bool( GetValue( Item( apdThinkNotifyCheck ) ) );
	gPrefsData.pdTreatSharesAsFriends = bool( GetValue( Item( apdTreatSharesAsFriendsCheck ) ) );
	gPrefsData.pdDisableCmdQ = bool( GetValue( Item( apdDisableCmdQCheck ) ) );
	gPrefsData.pdNewLogEveryJoin = bool( GetValue( Item( apdNewLogEveryJoinCheck ) ) );
	gPrefsData.pdNoMovieTextLogs = bool( GetValue( Item( apdNoMovieTextLogsCheck ) ) );
	
	SetCommandQStatus( not (gPlayingGame && gPrefsData.pdDisableCmdQ) );
	
# ifdef USE_OPENGL
	if ( GetValue( Item( apdOpenGLEnableCheck ) ) )		// GL enabled?
		{
		deleteNonOpenGLGameWindowObjects();
		gOpenGLEnableEffects = bool( GetValue( Item( apdOpenGLEffectsCheck ) ) );
		gPrefsData.pdOpenGLEnableEffects = gOpenGLEnable;
		if ( gOpenGLForceGenericRenderer != apdForceGenericRenderer
		||   gOpenGLPermitAnyRenderer != apdOpenGLPermitAnyRenderer )
			{
			if ( ctx )
				{
				glFinish();
				deleteOpenGLGameWindowObjects();
				DeleteTextureObjects();
				if ( not aglDestroyContext( ctx ) )
					ShowAGLErrors();
				
				ctx = nullptr;
				}
			gOpenGLForceGenericRenderer = apdForceGenericRenderer;
			gOpenGLPermitAnyRenderer = apdOpenGLPermitAnyRenderer;
			if ( apdOpenGLPermitAnyRenderer )
				gPrefsData.pdOpenGLRenderer = 2;
			else
			if ( apdForceGenericRenderer )
				gPrefsData.pdOpenGLRenderer = 1;
			else
				gPrefsData.pdOpenGLRenderer = 0;
			}
		if ( not ctx )
			{
			attachOGLContextToGameWindow();
			if ( ctx )
				{
				initOpenGLGameWindowFonts( ctx );
				setupOpenGLGameWindow( ctx, gPlayingGame );
				gOpenGLEnable = true;
				gPrefsData.pdOpenGLEnable = 1;
				}
			else
				{
				gOpenGLEnable = false;
				gPrefsData.pdOpenGLEnable = 0;
				}
			}
		else
			{
			gOpenGLEnable = true;
			gPrefsData.pdOpenGLEnable = 1;
			}
		}
	else
		{
		gOpenGLEnable = false;
		gPrefsData.pdOpenGLEnable = 0;
		if ( ctx )
			{
			glFinish();
			deleteOpenGLGameWindowObjects();
			DeleteTextureObjects();
			if ( not aglDestroyContext( ctx ) )
				ShowAGLErrors();
			ctx = nullptr;
			}
		}
# endif  // USE_OPENGL
	
	TouchHealthBars();
}


/*
**	SetCommandQStatus()
**
**	Add or remove the Cmd-Q menu key from the Quit menu item.
**	From OS 10.7 there are now multiple Quit items.
*/
void
SetCommandQStatus( bool enabled )
{
#if MAC_OS_X_VERSION_MAX_ALLOWED < 1070
	// command-IDs for the newer items, defined only in post-10.6 SDKs (in <CarbonEvents.h>)
	enum
		{
		kHICommandQuitAndDiscardWindows	= 'qudw',
		kHICommandQuitAndKeepWindows	= 'qukw',
		};
#endif  // < 10.7 SDK
	
	// what key equivalent to use: 'Q' or none?
	const UInt16 theKey = enabled ? 'Q' : '\0';
	
	// for each potential Quit command...
	const UInt32 cmds[] =
		{
		kHICommandQuit,
		kHICommandQuitAndDiscardWindows,
		kHICommandQuitAndKeepWindows,
		0
		};
	for ( const UInt32 * pcmd = cmds; *pcmd; ++pcmd )
		{
		// find the darned thing
		MenuRef quitMenu;
		MenuItemIndex quitItem;
		
		OSStatus result = ::GetIndMenuItemWithCommandID( nullptr, *pcmd,
								1, &quitMenu, &quitItem );
		
		// toggle its keyboard equivalent
		if ( noErr == result )
			{
			__Verify_noErr( SetMenuItemCommandKey( quitMenu, quitItem, false, theKey ) );
			}
		}
}


/*
**	HandleRejoinEvent
**
**	this custom AppleEvent is [normally] -only- sent to us as the last dying gasp of a
**	previous client incarnation that has successfully downloaded and installed
**	a newer client (namely, us!), as part of its LaunchParams.
**
**	Upon receipt, our job is to automatically rejoin the game. Why? Because if
**	there's a new client, then there's probably also some new image & sound data,
**	but we need to join up in order to [be instructed to] collect it.
**
**	The net effect is that the end user no longer has to hit "Join" _twice_ after an update.
*/
static OSErr
HandleRejoinEvent( const AppleEvent * /*evt*/, AppleEvent * reply, SRefCon /*refcon*/ )
{
#ifndef DEBUG_VERSION
# pragma unused( reply )
#endif
	
	OSStatus result;
	
	if ( gPrefsData.pdCharName[0]
	&&	 gPrefsData.pdAccount[0] )
		{
		// jump right in... (and finish the download)
		result = PerformAutoJoin();
		}
	else
		{
		result = errAEEventNotHandled;
		
#ifdef DEBUG_VERSION
			// TRANSLATIONS PLEASE
		const char * const reason =
			"Can't auto-join, because character or account name missing from prefs.";
		
		__Verify_noErr( ::AEPutParamPtr( reply, keyErrorString,
							typeUTF8Text, reason, strlen( reason ) ) );
#endif
		}
	
	return result;
}


/*
**	HandleConnStatusEvent()
**
**	return true if we're logged in
*/
static OSErr
HandleConnStatusEvent( const AppleEvent * /* evt */, AppleEvent * reply, SRefCon /* ud */ )
{
	AEDesc value = { typeNull, nullptr };
	if ( not gDoneFlag && gPlayingGame )
		value.descriptorType = typeTrue;
	else
		value.descriptorType = typeFalse;
	
	OSStatus result = ::AEPutParamDesc( reply, keyDirectObject, &value );

	__Verify_noErr( ::AEDisposeDesc( &value ) );
	return result;
}


/*
**	HandleMovieRecordEvent
*/
static OSErr
HandleRecordMovieEvent( const AppleEvent * evt, AppleEvent * reply, SRefCon /* refcon */ )
{
	// possible error states
	enum
		{
		errAlreadyRecording = 1,
		errAlreadyReading,
		errNoMovie
		};
	
	// extract recording control switch
	Boolean recordOn;
	OSStatus err = AEGetParamPtr( evt, keyDirectObject,
						typeBoolean, nullptr,
						&recordOn, sizeof recordOn, nullptr );
	
	// if we failed to get a parameter, just fail silently for now
	if ( err )
		return noErr;
	
	if ( recordOn )
		{
		// requested to start recording
		if ( CCLMovie::IsRecording() )
			err = errAlreadyRecording;
		else
		if ( CCLMovie::IsReading() )
			err = errAlreadyReading;
		else
			{
			DoRecordMovie();	// toggles -> on
			err = noErr;
			}
		}
	else
		{
		// requested to stop recording
		if ( CCLMovie::IsRecording() )
			{
			DoRecordMovie();	// toggles -> off
			err = noErr;
			}
		else
			err = errNoMovie;
		}
	
	const char * errMsg = nullptr;
	switch ( err )
		{
		case errAlreadyRecording:
			// "A movie is already being recorded."
			errMsg = _(TXTCL_MOVIE_ERR_ALREADY_RECORDING);
			break;
		
		case errAlreadyReading:
			// "Cannot record a movie while playing one."
			errMsg = _(TXTCL_MOVIE_ERR_NOT_RECORDING_WHILE_PLAYING);
			break;
		
		case errNoMovie:
			// "No movie is being recorded."
			errMsg = _(TXTCL_MOVIE_ERR_NO_MOVIE_RECORDING);
			break;
		}
	
	if ( errMsg )
		{
		::AEPutParamPtr( reply, keyErrorString, typeUTF8Text, errMsg, strlen(errMsg) );
		err = errAEEventNotHandled;
		}
	
	return err;
}


/*
**	InstallMovieHandler()
**
**	tell OS about our event handler for the Record Movie AppleEvent
*/
DTSError
InstallMovieHandler()
{
	DTSError result = AddAppleEventHandler( kClientAppSign, kRecordMovieEvent, 0,
			NewAEEventHandlerUPP( HandleRecordMovieEvent ) );
	__Check_noErr( result );
	
	return result;
}


/*
**	InstallAutoDownloadHandler()
**
**	tell OS about our event handler for the AutoDownload AppleEvent
*/
DTSError
InstallAutoDownloadHandler()
{
	DTSError result = AddAppleEventHandler( kClientAppSign, kJoinGameEvent, 0,
			NewAEEventHandlerUPP( HandleRejoinEvent ) );
	__Check_noErr( result );
	
	// and the one for the is-connected query
	if ( noErr == result )
		{
		result = AddAppleEventHandler( kClientAppSign, kConnectionStatusEvent, 0,
			NewAEEventHandlerUPP( HandleConnStatusEvent ) );
		__Check_noErr( result );
		}
	
	return result;
}


/*
**	ShowNetworkStats()
**
**	show the accumulated network statistics
*/
void
ShowNetworkStats()
{
	NetStatsDlg nsd;
	nsd.Run();
}


/*
**	NetStatsDlg::Init()
**
**	init the network statistics dialog
*/
void
NetStatsDlg::Init()
{
	// it's an OK button _and_ a cancel button!
	if ( HIViewRef btn = GetItem( Item( nsdOk ) ) )
		__Verify_noErr( SetWindowCancelButton( *this, btn ) );
	
	// set up a simple formatter, for slightly nicer integers
	if ( CFLocaleRef loc = CFLocaleCopyCurrent() )
		{
		nsdFormatter = CFNumberFormatterCreate( kCFAllocatorDefault,
							loc, kCFNumberFormatterNoStyle );
		CFRelease( loc );
		}
	if ( nsdFormatter )
		{
		// request thousands separators
		CFNumberFormatterSetFormat( nsdFormatter, CFSTR("#,##0") );
		}
	
	Update();
}


/*
**	NetStatsDlg::SetNumber()
**
**	overloaded version, for a nicer number format
*/
void
NetStatsDlg::SetNumber( const HIViewID& iid, int value )
{
	CFStringRef cs = nullptr;
	if ( nsdFormatter )
		cs = CFNumberFormatterCreateStringWithValue( kCFAllocatorDefault,
				nsdFormatter, kCFNumberIntType, &value );
	if ( cs )
		{
		SetText( iid, cs );
		CFRelease( cs );
		}
	else
		SetNumber( iid, value );	// failsafe
}


/*
**	NetStatsDlg::Update()
*/
void
NetStatsDlg::Update()
{
	nsdNumFrames = gNumFrames;
	
	SetNumber( Item( nsdSessionNumFrames ), gNumFrames );
	SetNumber( Item( nsdSessionLostFrames ), gLostFrames );
	SetNumber( Item( nsdTotalNumFrames ), gPrefsData.pdNumFrames );
	SetNumber( Item( nsdTotalLostFrames ), gPrefsData.pdLostFrames );
	
	int denom = gNumFrames + gLostFrames;
	int pctLost;
	if ( denom )
		pctLost = gLostFrames * 100 / denom;
	else
		pctLost = 0;
	
	char buff[ 32 ];
	snprintf( buff, sizeof buff, "%d%%", pctLost );
	SetText( Item( nsdSessionPercentLost ), buff );
	
	denom = gPrefsData.pdNumFrames + gPrefsData.pdLostFrames;
	if ( denom )
		pctLost = gPrefsData.pdLostFrames * 100 / denom;
	else
		pctLost = 0;
	snprintf( buff, sizeof buff, "%d%%", pctLost );
	SetText( Item( nsdTotalPercentLost ), buff );
	
	// also display crude packet latency figures
	// roundings include a factor of 0.5, to convert round-trip times to one-way.
	float sLat, mLat;
	gLatencyRecorder.GetStats( sLat, mLat );
	snprintf( buff, sizeof buff, "%g ms", int(sLat * 5) / 10.0 );
	SetText( Item( nsdSampleLatency ), buff );
	snprintf( buff, sizeof buff, "%g ms", int(mLat * 5) / 10.0 );
	SetText( Item( nsdMeanLatency ), buff );
}


/*
**	NetStatsDlg::Idle()
*/
void
NetStatsDlg::Idle()
{
	GameTickler::Idle();
	
	if ( gNumFrames != nsdNumFrames )
		Update();
}


/*
**	LatencyRecorder::LatencyRecorder()
*/
LatencyRecorder::LatencyRecorder() :
	mAckIndex( 0 ),
	mTotalLatency( 0 ),
	mNumLatencySamples( 0 )
{
	memset( ackTimes, 0, sizeof ackTimes );
}


/*
**	LatencyRecorder::RecordSentCmd()
**
**	remember the wall-clock time when we first sent this command number to server
*/
void
LatencyRecorder::RecordSentCmd( ulong cmdNum )
{
	if ( not cmdNum )
		return;
	
	// it's a circular buffer
	if ( mAckIndex >= kNumSavedAcks )
		mAckIndex = 0;
	
	// record it
	CmdAckRecord& r = ackTimes[ mAckIndex ];
	r.carNum = cmdNum & 0x0FF;
	r.carTimestamp = CFAbsoluteTimeGetCurrent();
	r.carLatency = 0;
	
	++mAckIndex;
}


/*
**	LatencyRecorder::Calc1Latency()
**
**	calculate elapsed time between when we sent a particular command to the server,
**	and when we received acknowledgement thereof.  Update the running totals.
*/
void
LatencyRecorder::Calc1Latency( int sampleIndex, CFAbsoluteTime when )
{
	CmdAckRecord * car = &ackTimes[ sampleIndex ];
	
	// convert to milliseconds
	uint latency = (when - car->carTimestamp) * 1000;
	car->carLatency = latency;
	
	++mNumLatencySamples;
	mTotalLatency += latency;
}


/*
**	LatencyRecorder::RecordRecvCmd()
**
**	called when we receive acknowledgment of a command
*/
void
LatencyRecorder::RecordRecvCmd( ulong cmdNum )
{
	if ( not cmdNum )
		return;
	
	// try to match it up with the pending commands
	CFAbsoluteTime now = CFAbsoluteTimeGetCurrent();
	for ( int i = 0; i < kNumSavedAcks; ++i )
		{
		if ( ackTimes[ i ].carNum == cmdNum
		&&	 0 == ackTimes[ i ].carLatency )
			{
			// found it!
			Calc1Latency( i, now );
			break;
			}
		}
}


/*
**	LatencyRecorder::GetStats()
**
**	getter function, for benefit of the NetworkStats dialog
*/
void
LatencyRecorder::GetStats( float& oSample, float& oMean ) const
{
	if ( mNumLatencySamples )
		{
		oMean = float(mTotalLatency) / mNumLatencySamples;
		
		uint sampleTotal = 0;
		for ( int i = 0; i < kNumSavedAcks; ++i )
			sampleTotal += ackTimes[ i ].carLatency;
		oSample = float(sampleTotal) / kNumSavedAcks;
		}
	else
		oMean = oSample = 0;
}


/*
**	RecordCmd()
**
**	a bit of a hack: allow GameWindow.cp to call RecordRecvCmd()
*/
void RecordCmd( ulong cmdNum );
void
RecordCmd( ulong cmdNum )
{
	gLatencyRecorder.RecordRecvCmd( cmdNum );
}


/*
**	DrawTextBox()
**
**	draws text by calling DrawThemeTextBox
**
**	text	what to draw
**	h, v	coordinates of reference point, same as DTSView::Draw()
**	just	justification, ditto
**	font	what meta-font to use -- see <Appearance.h>
*/
void
DrawTextBox( DTSView * win, CFStringRef text, DTSCoord h, DTSCoord v,
			 			 int just, ThemeFontID font )
{
	ThemeDrawState state = kThemeStateActive;
	if ( gInBack || win != gDTSApp->GetFrontWindow() )
		state = kThemeStateInactive;
	
	Point textDimen;
	SInt16 baseline;
	Rect box = { v, h, v, h };
	OSStatus err = ::GetThemeTextDimensions( text, font, state,
					false, &textDimen, &baseline );
	if ( noErr == err )
		{
		box.bottom -= baseline;
		box.top = box.bottom - textDimen.v;
		box.right = box.left + textDimen.h;
		}
	
	// account for justifications, and for the
	// rather annoying fact that DTSLib's justification codes
	// are not the same as the TextEdit ones.
	int hOffset = 0;
	switch ( just )
		{
		case kJustLeft:
			// normal case do nothing
#if kJustLeft != teJustLeft		// currently these both == 0, but semper paratus...
			just = teJustLeft;
#endif
			break;
		
		case kJustCenter:
			// center the box on the h coordinate
			hOffset = textDimen.h / 2;
			just = teJustCenter;
			break;
		
		case kJustRight:
			// shift it all the way over
			hOffset = textDimen.h;
			just = teJustRight;
			break;
		
		default:
#ifdef DEBUG_VERSION
			VDebugStr( "unknown justification code: %d", just );
#endif
			just = teJustLeft;		// just in case, as it were
			break;
		}
	
	box.left  -= hOffset;
	box.right -= hOffset;
	
	::DrawThemeTextBox( text, font, state, false, &box, just, nullptr );
}


/*
**	GetTextBoxWidth()
**
**	companion to above -- gets width of a CFString
*/
DTSCoord
GetTextBoxWidth( const DTSView * view, CFStringRef text, ThemeFontID font )
{
	int result = 0;

	ThemeDrawState state = kThemeStateActive;
	if ( gSuspended || view != gDTSApp->GetFrontWindow() )
		state = kThemeStateInactive;
	
	Point measure;
	SInt16 baseline;
	OSStatus err = GetThemeTextDimensions( text, font, state, false, &measure, &baseline );
	
	if ( noErr == err )
		result = measure.h;
	
	return result;
}


/*
**	SetBardVolume()
**
**	change master bard volume
*/
void
SetBardVolume( int inPct )
{
	if ( inPct < 0 )
		inPct = 0;
	else
	if ( inPct > 100 )
		inPct = 100;
	
#ifdef HANDLE_MUSICOMMANDS
	gTuneQueue.ChangeVolume( inPct );
#endif
}


#pragma mark About box

/*
**	Definitions
*/
const int	kNumLogoSteps		= 48;	// indirectly controls redraw frame-rate.
										// i.e., frames-per-second = (kNumLogoSteps / 2)

// How long it takes for the spinning yin-yang-thang to complete one rotation (in seconds)
#define	kAboutTimePerRev		(2 * kEventDurationSecond)

// How long between each animation frame? (inverse of frame rate)
#define	kAboutAnimDelay			(kAboutTimePerRev / kNumLogoSteps)

// How long to wait before starting the animation? (Classic DTSLib used 5 seconds)
#define	kAboutStartDelay		(0.9 * kEventDurationSecond)
//#define kAboutStartDelay		kAboutAnimDelay			// no appreciable startup delay

// todo: can frame-rate, # steps, and rotation-rate be made fully independent of each other?

// a float version of M_PI
#define F_PI					3.14159265F

// a new flag for Splash/ShowAboutBox()		[should move to About_dts.h]
enum
{
	kColoredLogo = 1U << 1		// enables fancier technicolor version
};


/*
**	class Spinny -- a little widget that draws an animated DT logo
**	inside a compositing window, such as the about box.
*/
class Spinny : public CarbonEventResponder , public Recurring
{
	typedef CarbonEventResponder SUPER;
	
public:
						Spinny( HIDialog *, HIViewRef ourView, bool bIsBW );
	virtual				~Spinny() { RemoveTimer();  }
	virtual OSStatus	HandleEvent( CarbonEvent& );
	virtual void		DoTimer( EventLoopTimerRef /*eltr*/ ) { AdvanceAnimIndex(); }
	
protected:
	void				AdvanceAnimIndex();
	void				DrawTheLogo( const CarbonEvent& );
	void				DrawLogoCG( CGContextRef, float width, float ht ) const;
	void				DrawYinYang( CGContextRef, bool ) const;
	
	HIViewRef		mView;
	HIDialog *		mParentDialog;
	int				curLogoIndex;
	bool			mInBoringBlackAndWhite;
};


/*
**	Spinny::Spinny()
**	constructor
*/
Spinny::Spinny( HIDialog * parent, HIViewRef ourView, bool bIsBW ) :
	mView( ourView ),
	mParentDialog( parent ),
	curLogoIndex( 0 ),
	mInBoringBlackAndWhite( bIsBW )
{
	// patch into the desired control-draw event
	if ( mView )
		InitTarget( GetControlEventTarget( mView ) );
	
	// we desire to receive draw-self events, and clicks
	const EventTypeSpec evts[] = {
			{ kEventClassControl, kEventControlDraw },
			{ kEventClassControl, kEventControlClick } };
	AddEventTypes( GetEventTypeCount( evts ), evts );
	
	InstallTimer( GetCurrentEventLoop(), kAboutStartDelay, kAboutAnimDelay );
}

/*
**	Spinny::HandleEvent()
**	dispatch our incoming carbon event(s)
*/
OSStatus
Spinny::HandleEvent( CarbonEvent& evt )
{
	// right now we only care about the draw event
	if ( kEventClassControl == evt.Class() )
		{
		switch ( evt.Kind() )
			{
			case kEventControlDraw:
				DrawTheLogo( evt );
				return noErr;
			
			case kEventControlClick:
				mParentDialog->Dismiss( kHICommandOK );
				return noErr;
			}
		}
	
	// let superclass have a crack at all the others
	return SUPER::HandleEvent( evt );
}


/*
**	Spinny::DrawTheLogo()
**	handle the kControlDraw event
*/
void
Spinny::DrawTheLogo( const CarbonEvent& evt )
{
	// extract our CGContext
	CGContextRef ctxt = nullptr;
	OSStatus err = evt.GetParameter( kEventParamCGContextRef,
							typeCGContextRef, sizeof ctxt, &ctxt );
	if ( noErr == err && ctxt )
		{
		// get target view's current dimensions
		HIRect box = CGRectMake( 0, 0, 128, 128 );		// failsafe default size
		if ( mView )
			__Verify_noErr( HIViewGetBounds( mView, &box ) );
		
		// do the real work
		DrawLogoCG( ctxt, box.size.width, box.size.height );
		}
}


// some dimensions for the logo. We are using a notional coordinate system 
// such that the entire logo fits into a 1000x1000 unit box.
// However at draw time, we'll scale the whole thing down so that it fits exactly
// into the frame of its HIView, as set in the nib.
const float LOGOBOX	= 1000.0F;		// max notional coordinate
const float HALFBOX	= LOGOBOX / 2;	// half of the max

const float RADIUS	= 495.0F;		// actual radius of the (round) logo.
										// this is a smidgeon less than HALFBOX, so that all
										// antialiasing & mitering (if any) stays within bounds

const float HalfRad	= RADIUS / 2;		// half of that radius
const float kCosine	= RADIUS * 0.866F;	// i.e. cos(30): used when drawing the triangle
const float kSine		= RADIUS / 2;		// sin(30), likewise

// these control the "ink" used when filling (B&W version) and/or stroking
const float kLogoColor	= 0.0F;		// 0 = black, 1 = white
const float kLogoAlpha	= 1.0F;		// 0 = transparent, 1 = opaque
const float kOuterStroke	= 6.0F;		// stroke width for outer border

// intensities and alpha of the RGB colors, for the non-B&W version
const float kYY_Primary	= 0.9F;		// "yin"'s RGB is { sec, sec, prim } -- blueish
const float kYY_Secondary = 0.2F;		// "yang"'s is { sec, prim, sec } -- greenish
const float kYY_Range		= kYY_Primary - kYY_Secondary;	// range between primary & secondary
const float kYY_Alpha		= 1.0F;		// colored version's alpha might (some day) differ from BW

const int kCounterRotateRate = 4;			// ratio of yy's rotation rate to triangle's rate

// anonymous namespace for purely file-local function(s)
namespace {

/*
**	SetLogoTrianglePath()
**	
**	appends the apexes (or apices) of the central triangle to the current path.
**	'clockwise' controls the "winding direction" of the added path segments.
**	also closes the path.
*/
void
SetLogoTrianglePath( CGContextRef ctx, bool clockwise )
{
	// OS 10.7 Lion seems to have a CG bug with winding directions
	if ( 10 == gOSFeatures.osVersionMajor && 7 == gOSFeatures.osVersionMinor )
		clockwise = not clockwise;
	
	CGContextMoveToPoint( ctx, 0, -RADIUS );
	if ( clockwise )
		{
		CGContextAddLineToPoint( ctx,  kCosine, kSine );
		CGContextAddLineToPoint( ctx, -kCosine, kSine );
		}
	else
		{
		CGContextAddLineToPoint( ctx, -kCosine, kSine );
		CGContextAddLineToPoint( ctx,  kCosine, kSine );
		}
	
	CGContextClosePath( ctx );
}
};	// end anon namespace


/*
**	Spinny::DrawLogoCG()
**	perform the actual drawing
*/
void
Spinny::DrawLogoCG( CGContextRef ctx, float width, float height ) const
{
	// xlate coords so center of logo is at (0,0), and scale to exactly fill the bounds
	CGContextTranslateCTM( ctx, width * 0.5F, height * 0.5F );	// was 64, 64
	CGContextScaleCTM( ctx, width / LOGOBOX, height / LOGOBOX );
	
	if ( mInBoringBlackAndWhite )
		{
		// old way: just black & white
		/*
		In the QuickDraw era, this was trivially easy: draw the main semicircle, then
		draw another smaller circle on one side, and erase a similar circle on the other side.
		Finally, XOR out the non-rotating triangle "delta" section.
		But Quartz doesn't do XORs.
		So what we do is this:
			1. clip to the _exterior_ of the triangle
			2. draw the positive part of the yinyang
			3. clip to the _interior_ of the triangle
			4. draw the positive yinyang, rotated 180 degrees for the inverted effect.
		*/
		
		// color of the "positive" sections
		CGContextSetGrayFillColor( ctx, kLogoColor, kLogoAlpha );
		
		CGContextSaveGState( ctx );
		
			// clip to exterior of triangle: add outer rect (clockwise path)...
			CGContextBeginPath( ctx );
			CGContextAddRect( ctx, CGRectMake( -HALFBOX, -HALFBOX, LOGOBOX, LOGOBOX ) );
			
			// ... and the inner triangle (CCW path, ergo subtracted from clip).
			// Note, because this window is composited, the HIView machinery has already
			// flipped the y-axis for us, and provided a QD-style coordinate system.
			// Hence a negative Y-coord is _upward_, above the centerline.
			SetLogoTrianglePath( ctx, false );
			CGContextClip( ctx );
			
			// draw the parts of the yinyang that lie outside the triangle
			DrawYinYang( ctx, false );
			
			// pop & push the gstate, to reset the clipping path
		CGContextRestoreGState( ctx );
		CGContextSaveGState( ctx );
			
			// clip to _interior_ of triangle
			CGContextBeginPath( ctx );
			SetLogoTrianglePath( ctx, true );
			CGContextClip( ctx );
			
			// draw the yinyang bits inside the triangle (rotated 180 degrees)
			DrawYinYang( ctx, true );
			
		// restore gstate again
		CGContextRestoreGState( ctx );
		}
	else
		{
		// new, colorized scheme:
		// paint yin in 50% green and yang in 50% blue, both clipped to inside the triangle
		// then swap colors and paint them again, this time clipped to outside the triangle.
		
		// counter-rotate the triangle, 1/4 as fast as the YY's
		float angle = curLogoIndex * 2 * F_PI / (kNumLogoSteps * kCounterRotateRate);
		
		// also cycle the colors alluringly
		float primGreen = kYY_Secondary + (kYY_Range * ( (1.0F + sinf( angle ))/2 ));
		float primBlue  = kYY_Secondary + (kYY_Range * ( (1.0f + cosf( angle ))/2 ));
		
		CGContextSaveGState( ctx );
		
			// first clip to exterior of triangle
			CGContextRotateCTM( ctx, -angle );
			CGContextBeginPath( ctx );
			CGContextAddRect( ctx, CGRectMake( -HALFBOX, -HALFBOX, LOGOBOX, LOGOBOX ) );
			SetLogoTrianglePath( ctx, false );
			CGContextClip( ctx );
			CGContextRotateCTM( ctx, angle );
			
			// blue yin
			CGContextSetRGBFillColor( ctx, 0, kYY_Secondary, primBlue, kYY_Alpha );
			DrawYinYang( ctx, false );
			
			// green yang
			CGContextSetRGBFillColor( ctx, 0, primGreen, kYY_Secondary, kYY_Alpha );
			DrawYinYang( ctx, true );
		
		// reset the clipping path
		CGContextRestoreGState( ctx );
		CGContextSaveGState( ctx );
		
			// now clip to inside of triangle
			CGContextRotateCTM( ctx, -angle );	// note negated angle
			CGContextBeginPath( ctx );
			SetLogoTrianglePath( ctx, true );
			CGContextClip( ctx );
			CGContextRotateCTM( ctx, angle );	// undo rotation
			
			// green yin
			CGContextSetRGBFillColor( ctx, 0, primGreen, kYY_Secondary, kYY_Alpha );
			DrawYinYang( ctx,  false );
			
			// blue yang
			CGContextSetRGBFillColor( ctx, 0, kYY_Secondary, primBlue, kYY_Alpha );
			DrawYinYang( ctx, true );
		
		CGContextRestoreGState( ctx );
		}
	
	// either way, lastly, stroke a circular black border around everything.
	CGContextSetGrayStrokeColor( ctx, kLogoColor, kLogoAlpha );
	CGContextSetLineWidth( ctx, kOuterStroke );
	CGContextMoveToPoint( ctx, RADIUS, 0 );		// positive X axis
	CGContextAddArc( ctx, 0, 0, RADIUS, 0, 2 * F_PI, 1 );		// a complete (clockwise) circle
	CGContextStrokePath( ctx );
}


/*
**	Spinny::DrawYinYang()
**	draw the curvy part of the logo
**	(or rather, half of it: this routine actually gets called (at least) twice per frame,
**	with the CTM rotated 180 degrees after each call. In other words, one call
**	produces the "yin" and the other makes the "yang".)
**	Each half-logo is painted using whatever fill color was last set by our caller.
**	The colored codepath calls this four times per frame, once for each combination of...
**	  -	green vs blue fill color
**	  - clipping inside vs outside the triangle
*/
void
Spinny::DrawYinYang( CGContextRef ctx, bool invert ) const
{
	// handy mnemonics for CGContextAddArc()'s "clockwise" argument
	enum {
		kCCW = 0,
		kCW = 1
		};
	
	// determine the current angle, based on number of steps
	float angle = curLogoIndex * 2 * F_PI / kNumLogoSteps;
	if ( invert )
		angle += F_PI;
	// (at the end of the day, 'angle' can end up being much higher than 2*pi. But that's OK.)
	
	CGContextRotateCTM( ctx, angle );
	
	// draw the "positive" lower half-circle
	CGContextBeginPath( ctx );
	CGContextMoveToPoint( ctx, RADIUS, 0 );
	CGContextAddArc( ctx, 0, 0, RADIUS, 0, F_PI, kCW );
	
	// exclude the negative small right-hand circle scallop
	CGContextAddArc( ctx, -HalfRad, 0, HalfRad, F_PI, 0, kCCW );
	
	// include the positive small left-hand circle bump
	CGContextAddArc( ctx, HalfRad, 0, HalfRad, F_PI, 0, kCW );
	
	// paint it
	CGContextFillPath( ctx );
	
	// undo the rotation
	CGContextRotateCTM( ctx, -angle );
}


/*
**	Spinny::AdvanceAnimIndex()
**	bump the frame counter and request a redraw
*/
void
Spinny::AdvanceAnimIndex()
{
	++curLogoIndex;
	
	// we let the index go up to (kCounterRotateRate * numSteps), because the counter-rotating
	// inner triangle spins considerably slower than the yinyangthang. hence we do not
	// want to reset curLogoIndex to 0 until the triangle has completed one whole spin.
	// (otherwise it snaps back prematurely)
	if ( curLogoIndex < 0  ||  curLogoIndex >= (kCounterRotateRate * kNumLogoSteps) )
		curLogoIndex = 0;
	
	if ( mView )
		__Verify_noErr( HIViewSetNeedsDisplay( mView, true ) );
}

#pragma mark -


/*
**	AboutBox::~AboutBox()
*/
AboutBox::~AboutBox()
{
	delete abSpinny;
}


/*
**	AboutBox::Init()
*/
void
AboutBox::Init()
{
	// determine year
	int year = 2016;
	if ( true )
		{
		// we need the build date, NOT the runtime date
		const char * buildDate = __DATE__;		// e.g. "Dec 25 2016"
		
		// find rightmost ' ' in __DATE__; everything thereafter is the year
		const char * yp = strrchr( buildDate, ' ' );
		if ( yp && *yp && yp[1] )
			year = atoi( yp + 1 );
		}
	
	// cons up the fine print: "blah, blah, version: v%@ (%d) blah (c) 1990-%d blah"
	CFStringRef fmt = CFCopyLocalizedString( CFSTR("ABOUT_BOX_BLURB"),
						CFSTR("fine print for the about box") );
	
#ifdef DEBUG_VERSION
	CFStringRef vStr = CFSTR(kFullVersionString " [debug]");
#else
	CFStringRef vStr = CFSTR(kFullVersionString);
#endif
	
	int buildNo = 0;
	if ( CFTypeRef tr = CFBundleGetValueForInfoDictionaryKey( CFBundleGetMainBundle(),
							kCFBundleVersionKey ) )
		{
		buildNo = CFStringGetIntValue( static_cast<CFStringRef> ( tr ) );
		}
	
	if ( CFStringRef blurb = CFStringCreateFormatted( fmt, vStr, buildNo, year ) )
		{
		SetText( Item( abBlurb ), blurb );
		CFRelease( blurb );
		}
	if ( fmt )
		CFRelease( fmt );
	
#if USE_ABOUT_PNG_FILE
	SetAboutPicture();
#endif
	
	HIViewRef spinner = GetItem( Item( abSpinner ) );
	abSpinny = NEW_TAG("Spinny") Spinny( this, spinner, true );
}


# if USE_ABOUT_PNG_FILE
/*
**	AboutBox::SetAboutPicture()
**
**	install the "About.png" picture into the dialog's HIImageView
*/
void
AboutBox::SetAboutPicture() const
{
	CGImageRef img = nullptr;
	
	// locate the bundled image file
	if ( CFURLRef uu = CFBundleCopyResourceURL( CFBundleGetMainBundle(),
							CFSTR("About"), CFSTR("png"), nullptr ) )
		{
		// ask ImageIO to read that file, and create a CGImageRef from it
		if ( CGImageSourceRef src = CGImageSourceCreateWithURL( uu, nullptr ) )
			{
			img = CGImageSourceCreateImageAtIndex( src, 0, nullptr );
			CFRelease( src );
			}
		
		CFRelease( uu );
		}
	
	// if we successfully got an image, install it into the dialog's image-view
	__Check( img );
	if ( img )
		{
		if ( HIViewRef pictVu = GetItem( Item( abPicture ) ) )
			{
			__Verify_noErr( HIImageViewSetImage( pictVu, img ) );
			}
		
		CFRelease( img );
		}
}
# endif  // USE_ABOUT_PNG_FILE

