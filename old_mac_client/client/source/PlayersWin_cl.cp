/*
**	PlayersWin_cl.cp		ClanLord
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
#include "VersionNumber_cl.h"
#include "Commands_cl.h"
#include "ListView_cl.h"
#include "Macros_cl.h"
#include "SendText_cl.h"


#define ALLOW_MACRO_CLICKS		1

// placeholder -- someday this might be its own separate text style
#define kMsgShareLogoff			kMsgFriendLogoff

/*
**	Entry Routines
*/
/*
void		CreatePlayersWindow( const DTSRect * pos );
void		RequestPlayersData();
void		ResetPlayers();
bool		ParseInfoText( const char * text, BEPPrefixID id, MsgClasses * );
bool		ParseThinkText( DescTable * target );
void		SelectPlayer( const char * name, bool toggle = true );
int			GetPlayerIsFriend( const DescTable * desc );
int			GetPlayerIsFriend( const char * name );
void		SetPlayerIsFriend( const DescTable * desc, int friends );
bool		GetPlayerIsDead( const DescTable * desc );
void		SetPlayerIsDead( const DescTable * desc, bool dead );
bool		GetPlayerIsClan( const DescTable * desc );
void		SetPlayerIsClan( const DescTable * desc, bool clan );
bool		GetPlayerIsSharing( const DescTable * desc );
void		SetPlayerIsSharing( const DescTable * desc, bool sharing );

bool		GetPlayerIsSharee( const DescTable * desc );
bool		GetPlayerIsSharee( const char * name );
void		SetPlayerIsSharee( const DescTable * desc, bool sharing );
*/


/*
**	Global variables
*/
const DTSColor gFriendColors[ kFriendFlagCount ] =
{
	DTSColor( 0xFFFF, 0xFFFF, 0xFFFF ),		// 0 none (white, but not drawn)
	DTSColor( 0xD800, 0x4000, 0x4000 ),		// 1 label 1 (red)
	DTSColor( 0xE000, 0x9E00, 0x2000 ),		// 2 label 2 (orange)
	DTSColor( 0x5A00, 0xA500, 0x5A00 ),		// 3 label 3 (green)
	DTSColor( 0x4000, 0x2000, 0xE000 ),		// 4 label 4 (blue)
	DTSColor( 0x8900, 0x5A00, 0xB900 ),		// 5 label 5 (purple)
	DTSColor( 0xAD00, 0xAD00, 0xAD00 ),		// 6 block (medium grey)
	DTSColor( 0xD700, 0xD700, 0xD700 )		// 7 ignore (light grey)
};


/*
**	Definitions
*/

// pictIDs we might not want to save persistently, client-side
const int kNewbieRacelessPlayerPict = 22;			// hasn't decided yet
const int kBoatPlayerPict 			= 537;			// transient
const int kNethackPlayerPict		= 3371;			// ditto (from 2002/4 April Fools)

const int kBardPlayerMarkPict		= 1289;

const int kSavedPlayersStaleDelay	= 10;			// in seconds
const int kAutoCommandDelay			= 180;			// in ticks

const int kNameColumnWidth			= 175;
const int kFlagColumnWidth			= 12;
const int kMaxPlayersContWidth		= kNameColumnWidth + 1 + (kFlagColumnWidth * 2);	// ~ 200
const int kResetAreaHeight			= 16;
const int kFallenInfoHeight			= 16;
const int kCountsAreaHeight			= 15;
const int kTotalStaticHeights		= kResetAreaHeight + kFallenInfoHeight + kCountsAreaHeight;

const int kPlayerInfoHeight			= 18;			// height of list cells
const int kMinPlayersContHeight		= 8 * kPlayerInfoHeight + kTotalStaticHeights;	// min. 8 lines
const int kMaxPlayersContHeight		= 40 * kPlayerInfoHeight + kTotalStaticHeights;	// max. 40 lines

const int kSBarWidth				= 15;
const int kMaxPlayersWinWidth		= kMaxPlayersContWidth + kSBarWidth;
const int kMaxPlayersWinHeight		= kMaxPlayersContHeight;
const int kMinPlayersWinWidth		= // 175 + kSBarWidth;		// TXTCL_DBLCLICK_TO_RESET_LIST
									  kMaxPlayersWinWidth;		// min width == max width

const int kMaxPWIconSize			= 16;		// never draw icons bigger than this


const char kBEPPChar = '\xC2';		// this character precedes all BEPPs
			// (In MacRoman encoding, it's the "not sign": option + "l" on US keyboards)

enum
	{
	playerStatus_Idle		= 0,
	playerStatus_Who,
	playerStatus_DidWho,
	playerStatus_Share,
	playerStatus_Info
	};


//
// These should be in the CLS_World order. Just to be sure to
// "hide" the classes you want to hide by replacing them with ""
//
const char * const kPlayerClass[] =
{
	TXTCL_PLAYERCLASS_EXILE,	// "an exile",
	TXTCL_PLAYERCLASS_HEALER,	// "a Healer",
	TXTCL_PLAYERCLASS_FIGHTER,	// "a Fighter",
	TXTCL_PLAYERCLASS_MYSTIC,	// "a Mystic",
	"",							// "an unknown",
	"",							// "a Ranger" (old type; to be reused -- don't confuse w/#20)
	"",							// "a Spirit Lord",
	TXTCL_PLAYERCLASS_GM,		// "a Game Master",
	"",							// "a Merchant",
	"",							// "the Sheriff",
	"",							// "a public servant",
	"",							// "a Monster",
	TXTCL_PLAYERCLASS_APMYSTIC, // "an Apprentice Mystic",
	"",							// "a Mist Weaver",
	"",							// "a Mist Spinner",
	"",							// "a Mist Walker",
	"",							// "a Mist Matron",
	TXTCL_PLAYERCLASS_JMMYSTIC, // "a Journeyman Mystic",
	TXTCL_PLAYERCLASS_BLOODMAGE, // "a Bloodmage",
	TXTCL_PLAYERCLASS_CHAMPION,	// "a Champion",
	TXTCL_PLAYERCLASS_RANGER,	// "a Ranger" (modern type)
#ifndef ARINDAL
	"",							// "a Seer",
	"",							// "a Lunaar",
	"",							// "a Geomancer",
#endif  // not ARINDAL
	nullptr
};

//
// These need to exactly match the race names in CLS_World
// but "hiding" is allowed, as with kPlayerClass[] above.
//
const char * const kPlayerRaces[] =
{
	TXTCL_RACE_UNDISCLOSED,		// "not disclosed ",
	TXTCL_RACE_ANCIENT,			// "an Ancient",
	TXTCL_RACE_HUMAN,			// "a Human",
	TXTCL_RACE_HALFLING,		// "a Halfling",
	TXTCL_RACE_SYLVAN,			// "a Sylvan",
	TXTCL_RACE_PEOPLE,			// "of the People",
	TXTCL_RACE_THOOM,			// "a Thoom",
	TXTCL_RACE_DWARF,			// "a Dwarf",
	TXTCL_RACE_GHORAK_ZO,		// "a Ghorak Zo",
	"",							// "a Darshak",
	"",							// "a Spirit Lord",
	"",							// "a Monster",
	"",							// "Unknown"
	"",							// "Orga"
#ifdef ARINDAL
	TXTCL_RACE_SIRRUSH,			// "a Sirrush",
	TXTCL_RACE_AZCATL,			// "an Azcatl",
	TXTCL_RACE_LEPORI,			// "a Lepori",
#endif // ARINDAL
	nullptr
};

const char * const kPlayerGenders[] =
{
	TXTCL_GENDER_OTHER,			// "other",
	TXTCL_GENDER_MALE,			// "male",
	TXTCL_GENDER_FEMALE,		// "female",
	TXTCL_GENDER_UNKNOWN,		// "unknown",
	nullptr
};


// Friends label names, all lower-case
const char * const sFriendLabelNames[] =
{
	TXTCL_COLOR_NONE,		// "none",
	TXTCL_COLOR_RED,		// "red",
	TXTCL_COLOR_ORANGE,		// "orange",
	TXTCL_COLOR_GREEN,		// "green",
	TXTCL_COLOR_BLUE,		// "blue",
	TXTCL_COLOR_PURPLE,		// "purple",
	TXTCL_COLOR_BLOCKED,	// "blocked",
	TXTCL_COLOR_IGNORED		// "ignored"
};


// bit positions and masks for PlayerNode::pnFlags
enum
	{
	kGodShift		= 0,	// gm level 0-15 (more than actually necessary)
//	kSharingShift	= 4,	// boolean: player is sharing TO me
//	kDeadShift		= 5,	// boolean: player is fallen, last we heard
//	kUnusedShift	= 6,	// 2 bits unused
//	kClanShift		= 8,	// boolean: player is same clan as me
//	kShareeShift	= 9,	// boolean: player is being shared FROM me
//  kBEWhoShift		= 10,	// boolean: player has been "/be-who'ed"
	kFriendShift	= 11	// 3 bits friend/label value (kFriendNone..kFriendIgnore)
		// remaining 17 bits still available at low rates: inquire within!
	};

enum
	{
	kGodMask		= 0x0000000F,
	kSharingMask	= 0x00000010,
	kDeadMask		= 0x00000020,
//	kUnusedMask		= 0x000000C0,	// these 2 bits can be reused
	kClanMask		= 0x00000100,
	kShareeMask		= 0x00000200,
	kBEWhoMask		= 0x00000400,
	kFriendMask		= 0x00003800
	};

// the kBEWho flag bit indicates that this player has been enumerated via "\be-who"
// I use it to know when to stop sending \be-who commands to the host.

const uchar gNewbieDimColors[ kNumCustColors ] =
	{
	// color-index values, not 555 pseudo-RGB
	135, 171,   1,   8,  15,   1,   7,   8,   1,   8,
	 15,   8,  15,   7,   1,  43,   1,  43,  36, 108
 // ,35,  35,  35,  35,  35,  35,  35, 249,  35, 255
 	};


// forward declaration
class PlayerNode;

/*
**	PlayerInfo class -- info about a player which we retain in CL_Players file
**	this structure is packed to be saved in the player keyfile
*/
#pragma pack( push, 2)

class PlayerInfo
{
	enum
		{
		kHeaderID		= '\0vn#',
		kHeaderIndex	= 0,
		
		kVersion		= 0x0001,
		kDeleteDelay	= 30 * 24 * 3600		// a month (in seconds)
		};
public:
	enum
		{
		class_Bard		= 0x80U,
		
		class_Mask		= ~(class_Bard)
		};
	uint32_t		mGender : 8,
					mRace	: 8,
					mClass	: 8,						// profession, skills
					mClan	: 8;
	uint32_t		mReserved;
	int32_t			mPictID;							// Picture ID of the guy
	uint32_t		mLastSeen;							// Mac GetDateTime() when last seen
	uchar			mColors[ kNumPlyColors + 1 ];		// mColors[0] = # colors
	char			mName[ kMaxNameLen + 1 ];			// mName[0] = len (ie, a pascal string)
	
	// because DTSKeyFiles are indexed by 4-byte types & 4-byte IDs, rather than
	// arbitrary strings (e.g. player names), we needed a way to map names to keytypes.
	// We settled on a scheme where the high-order byte is the length of the name,
	// and rest is the first three bytes of the name itself. This avoids most key-type
	// collisions, but doesn't totally eliminate them -- but that's OK; the full name
	// is available for disambiguation in the data itself (in 'mName'). So if you have
	// two players, say "Gaia" and "Gail", both will share the same KeyType ('\4Gai'),
	// but they'll have different KeyIDs.
	// Some keytypes are "special" because they don't correspond to actual player names.
	// The primary example is the version record, which is 'kHeaderID'( 0 ).
	// All such special keytypes must have 0 as their high-order byte.
	
static DTSKeyType	MakeNameKey( const char * name );
static bool			IsSpecialKey( DTSKeyType key )
						{
						// is this a special (non-player) key, like kHeaderID?
						return (key & 0xFF000000U) == 0;
						}
static bool			FindPlayerID( const char * name, DTSKeyType& outType,
						DTSKeyID& outID, PlayerInfo& outInfo );

static void			OpenFile();
static void			CloseFile();
static DTSError		Convert( int inVersion = kVersion );
static DTSError		Stats( char * outtext );
static DTSError		CompressFile();

static DTSError		SavePlayer( const PlayerNode * player );
static bool			LoadPlayer( PlayerNode * player );

static DTSError		sFileError;
static DTSKeyFile	sFile;

					// constructor
			PlayerInfo()
				:
				mGender( 0 ),
				mRace( 0 ),
				mClass( 0 ),
				mClan( 0 ),
				mReserved( 0 ),
				mPictID( kNewbieRacelessPlayerPict ),
				mLastSeen( 0 )
			{
				mName[ 0 ]   = '\0';
				mColors[ 0 ] = sizeof gNewbieDimColors;
				memcpy( &mColors[1], gNewbieDimColors, sizeof gNewbieDimColors );
			}
};
#pragma pack( pop )


/*
**	PlayerNode class - information about a player
**
**	It's a pretty odd data structure -- both a self-balancing binary tree AND
**	a doubly-linked list, simultaneously. The tree part (pnLeft, pnRight) is kept in
**	alphabetical name order, except that each time a node is successfully looked up,
**	that node is "bubbled up" to become the root of the tree. Thus, recently- or
**	frequently-accessed nodes tend to migrate closer to the root.
**	The list part (pnNext, pnPrev) is also kept in the same order, to enable easy sequential
**	access of the list.
**
**	One can't help feeling that this is all tremendously overengineered for our needs.
**	We rarely if ever see >100 players online, so the linear lookup costs of a plain
**	list would never become onerous. What's more, one can easily imagine desiring to
**	sort the player-list by other criteria than names: e.g., by clan or class or date-fell...
**  but the present structure makes that exceedingly tough.
*/
class PlayerNode
{
public:
	PlayerNode *	pnLeft;
	PlayerNode *	pnRight;
	PlayerNode *	pnNext;
	PlayerNode *	pnPrev;
	
	char			pnName[ kMaxNameLen ];
	char			pnLocFell[ 32 ];
	char			pnKillerName[ 32 ];
	uint			pnFlags;
	int				pnNum;			// display order
	int				pnDateFell;		// a truncated CFAbsoluteTime
	
	PlayerInfo		pnInfo;
	DTSKeyID		pnPictCacheID;
	
	// constructor/destructor
	explicit		PlayerNode( const char * name ) :
						pnLeft( nullptr ),
						pnRight( nullptr ),
						pnNext( nullptr ),
						pnPrev( nullptr ),
						pnFlags( 0 ),
						pnNum( 0 ),
						pnPictCacheID( kNewbieRacelessPlayerPict )
					{
					StringCopySafe( pnName, name, sizeof pnName );
					
					pnLocFell[0] 	=
					pnKillerName[0]	= '\0';
					}
	
/* virtual */		~PlayerNode() {}
	
	bool			UpdateFlags( uint flags, uint mask );
	
	// misc accessors
	int				Friendship() const { return (pnFlags & kFriendMask) >> kFriendShift; }
	
	bool			IsFriendly() const
						{
						uint friends = Friendship();
						return friends >= kFriendLabel1 && friends <= kFriendLabelLast;
						}
	
	bool			IsNewbie() const		// looks like a newbie?
						{
						return kNewbieRacelessPlayerPict == pnInfo.mPictID
							&& sizeof gNewbieDimColors == pnInfo.mColors[0]
							&& 0 == memcmp( &pnInfo.mColors[1], gNewbieDimColors,
										sizeof gNewbieDimColors );
						}
	
	bool			NeedsInfo() const		// still need to do /be-info on this guy?
						{
						// if we don't yet know his race, then we should get the info,
						// except, don't bother for _very_ new newbies, since they won't
						// have had a chance to pick a race yet.
						return 0 == pnInfo.mRace && not IsNewbie();
						}
};


//
//	PlayersList - list container for player nodes in the Players window
//
class PlayersList : public ListView
{
public:
	int					plSelectNum;
	PlayerNode *		plSelect;
	UInt32				plLastKeyTime;
	
	PlayersList() :
		plSelectNum( -1 ),
		plSelect( nullptr ),
		plLastKeyTime( 0 )
		{}
	
	// overloaded routines
	virtual void		DrawNode( int inIndex, const DTSRect& inRect, bool bActive );
	void				Draw1Icon( const ImageCache *, DTSRect& dst, int iconSize );
	void				Draw1Mobile( const ImageCache * icon, DTSRect& dst, bool bFallen );
	
	virtual bool		Click( const DTSPoint& start, ulong time, uint modifiers );
	virtual bool		ClickNode( int inIndex, const DTSPoint& start, ulong time, uint mods );
	void				HandleNameClick( PlayerNode * player, uint modifiers,
							int listNum, int delay );
	
	virtual bool		KeyStroke( int ch, uint modifiers );
	bool				NormalKeyStroke( int ch );
	
	static DTSKeyID		PictureIDForClass( int pClass );
};

static PlayersList	gPlayersList;


//
//	PlayersContent - contents of the players-list window
//
class PlayersContent : public DTSView
{
public:
	PlayersList *		pcPlayersList;
	
	ulong				pcLastClickTime;
	
	DTSRect				pcTopBox;
	DTSRect				pcBottomBox;
	
	bool				pcActive;
	bool				pcShowLocation;		// vs show killer
	
						PlayersContent();
//	virtual				~PlayersContent() {}
	
	// overloaded routines
	virtual void		DoDraw();
	virtual void		Size( DTSCoord h, DTSCoord v );
	virtual bool		Click( const DTSPoint& start, ulong time, uint modifiers );
	DTSPoint			BestSize( DTSPoint pt ) const;
	
	void				Activate()		{ pcActive = true; }
	void				Deactivate()	{ pcActive = false; }
};


#pragma mark class PlayersWindow
//
//	PlayersWindow - the window
//
class PlayersWindow : public DTSWindow, public CarbonEventResponder
{
public:
	PlayersContent		pwContent;
	
	// overloaded routines
	virtual DTSError	Init( uint kind );
	virtual void		Activate();
	virtual void		Deactivate();
	virtual void		Size( DTSCoord h, DTSCoord v );
	
	virtual bool		KeyStroke( int ch, uint modifiers );
	
	OSStatus			DoWindowEvent( const CarbonEvent& );
	virtual OSStatus	HandleEvent( CarbonEvent& );

	void				AdjustCount();
};


//
// list of friends who were online when we joined
//
class FriendsList : public DTSLinkedList<FriendsList>
{
	explicit				FriendsList( const PlayerNode * );
	
protected:
	const PlayerNode * 		mPlayer;
	static FriendsList *	sFriendsListRoot;

public:
	static void			AddFriend( const PlayerNode * inFriend );
	static void 		PrintFriends( int label );
};


/*
**	Internal Routines
*/
static void				ClearSelection();
static void				RemoveAllPlayersFrom( PlayerNode ** queue );
static PlayerNode *		FindNumInList( int num );
static bool				IsPlayerBeingShown( PlayerNode * player, int num );
static void				NativeToBigEndian( PlayerInfo& info );
static void				BigToNativeEndian( PlayerInfo& info );


/*
**	Internal Variables
*/
	// gPlayersRoot is the current root of the node tree, i.e. the most-recently-looked-up
	// player. It changes very frequently.
static PlayerNode *			gPlayersRoot;

	// gPlayersHead/Tail are the ends of the doubly-linked-list; they don't change often
	// (unless an alphabetically-extreme player is added or removed)
static PlayerNode *			gPlayersHead;
static PlayerNode *			gPlayersTail;

	// gPlayersSaveHead is where we park the old node tree during a player-window Reset.
	// Nodes are "revivified" from the save-head back to the real tree during the /be-who
	// processing that follows shortly after the reset.
static PlayerNode *			gPlayerSaveHead;

	// timestamp of nodes in save-head, to prevent revival of stale nodes
static UInt32				gPlayerSaveTime;

static int					gNumPlayers;			// # players now online
static bool					gPlayerIsAGod;			// we think the user's player is a GM
static int					gNumShares;				// total inbound shares
static int					gNumSharees;			// total outbound ditto

	// The player window runs a state machine that periodically sends invisible
	// /BE-XXX query commands to the server, and updates itself accordingly.
	// These next few vars control that machinery.
static bool					gNeedWho;				// true if we need to emit more /be-who cmds
static ulong				gLastAutoCommandTime;	// [ticks] throttle for BE-X cmds; musn't spam
static int					gPlayerListStatus = playerStatus_Idle;
bool						gPlayersListIsStale = true;

	// class statics
DTSError					PlayerInfo::sFileError;
DTSKeyFile					PlayerInfo::sFile;
FriendsList *				FriendsList::sFriendsListRoot;


/*
**	CreatePlayersWindow()
**
**	Create and show the players window.
*/
void
CreatePlayersWindow( const DTSRect * pos )
{
	// create the window
	PlayersWindow * win = NEW_TAG("PlayersWindow") PlayersWindow;
	CheckPointer( win );
	gPlayersWindow = win;
	
	// create window of type document with close, zoom, and grow boxes.
	DTSError result= win->Init( kWKindDocument
								| kWKindCloseBox | kWKindZoomBox | kWKindGrowBox
								| kWKindGetFrontClick
								| kWKindLiveResize );
	if ( result )
		return;
	
	// init the list
	gPlayersList.Init( &win->pwContent, win, kPlayerInfoHeight );
	
	// set the title and position it
	win->SetTitle( _(TXTCL_TITLE_PLAYERS) );	// "Players"
	win->SetZoomSize( kMaxPlayersWinWidth, kMaxPlayersWinHeight );
	int top  = pos->rectTop;
	int left = pos->rectLeft;
	win->Move( left, top );
	DTSPoint sz( pos->rectRight - left, pos->rectBottom - top );
	sz = win->pwContent.BestSize( sz );
	win->Size( sz.ptH, sz.ptV );
	
	win->pwContent.Show();
	gPlayersList.Show();
	win->Show();
	
	ResetPlayers();
}


/*
**	MyGetDateTime()
**
**	timestamp source for player-info records
*/
static void
MyGetDateTime( UInt32 * when )
{
#if 0
	// GetDateTime() is deprecated.
	::GetDateTime( when );
#elif 0
	// this yields the same result as GetDateTime(), and therefore depends on
	// the current time zone and daylight savings and etc.
	__Verify_noErr( UCConvertCFAbsoluteTimeToSeconds( CFAbsoluteTimeGetCurrent(), when ) );
#else
	// so from v1055 on, let's use a [monotonically-increasing] number based only on UTC
	*when = static_cast<UInt32>( CFAbsoluteTimeGetCurrent() );
#endif  // 0
}


/*
**	InitPlayers()
**	get ready for action, just after sign-on (or start of movie playback)
*/
void
InitPlayers()
{
	ResetPlayers();
	PlayerInfo::OpenFile();
}


/*
**	DisposePlayers()
**	close up shop, just after signoff (or movie ends)
*/
void
DisposePlayers()
{
	PlayerInfo::CloseFile();
}


/*
**	ResetPlayers()
**
**	Clear the players list and set the state back to nothing.
*/
void
ResetPlayers()
{
	// remove possible state list
	RemoveAllPlayersFrom( &gPlayerSaveHead );
	gPlayerSaveTime		= 0;
	
	gNumPlayers			= 0;
	gNumSharees			= 0;
	gNumShares			= 0;
	gPlayerIsAGod		= false;
	gPlayersListIsStale	= true;
	
	// save the old player-info tree, so that we can salvage its nodes during the next /be-who
	gPlayerSaveHead		= gPlayersRoot;		// save it
	MyGetDateTime( &gPlayerSaveTime );
	
	if ( gPlayerSaveHead )
		ShowMessage( _(TXTCL_SAVE_PLAYER_LIST) );	// "* Save player list"
	
	// reset descriptor table too; we are supposed to have freed!
	for ( int d = 0; d < kDescTableSize; ++d )
		gDescTable[d].descPlayerRef = nullptr;
	
	gPlayersTail 		= nullptr;
	gPlayersRoot 		= nullptr;
	gPlayersHead		= nullptr;
	
	ClearSelection();
	
	gPlayersList.lvNumNodes = 0;
	
	gPlayersWindow->Invalidate();
	gPlayersList.AdjustRange();
}


/*
**	CompressPlayers()
**	perform a full rebuild of the CL_Players file
*/
static void
CompressPlayers()
{
	// "Rebuild the CL_Players file?"
	int result = GenericOkCancel( _(TXTCL_REBUILD_CL_PLAYERS) );
	if ( kGenericOk == result )
		{
		ResetPlayers();
		PlayerInfo::CompressFile();
		}
}


/*
**	RequestPlayersData()
**
**	This function is called by the main application loop once per frame.
**	If the current Player Info is stale, we'll send a series of commands, once per
**	second, to refresh the data.
**	A state machine determines whether to request /who, /share, or /info data
**	(or just to do nothing)
*/
void
RequestPlayersData()
{
	// sideband signal from movie playback
	if ( gPlayersListIsStale )
		{
		// restart the state machine from scratch
		gPlayerListStatus 		= playerStatus_Who;
		gLastAutoCommandTime	= 0;
		gNeedWho				= true;
		gPlayersListIsStale		= false;
		}
	
	ulong currentTime = GetFrameCounter();
	switch ( gPlayerListStatus )
		{
			//
			// Idle: scour the player list for a guy that we don't know about.
			// Goes to state_Info once we get an answer from "/be-info".
			//
		case playerStatus_Idle:
			{
				// ...but only at a measured, dignified pace
			if ( currentTime < gLastAutoCommandTime + kAutoCommandDelay )
				break;
			
			gLastAutoCommandTime = currentTime;
			
			// scan for newcomers
			for ( const PlayerNode * cur = gPlayersHead; cur; cur = cur->pnNext )
				{
				if ( cur->NeedsInfo() )
					{
					// found a player to inquire about
					
					SendCommand( "be-info", cur->pnName, true );
#ifdef DEBUG_VERSION
//					ShowMessage( "be-info '%s'", cur->pnName );
#endif
					break;
					}
				}
			}
			break;
		
			//
			// Who: let's populate the player list. Send one or more /be-who commands.
			// Goes to state_DidWho as soon as we've seen everybody.
			//
		case playerStatus_Who:
				// piano, piano
			if ( currentTime < gLastAutoCommandTime + kAutoCommandDelay )
				break;
			
			gLastAutoCommandTime = currentTime;
			
			if ( gNeedWho )
				{
				SendCommand( "be-who", nullptr, true );
#ifdef DEBUG_VERSION
//				ShowMessage( "be-who" );
#endif
				}
			break;
		
			//
			// DidWho: an ideal moment to print out the list of online friends,
			// now that the /be-who has completed.
			// Goes immediately to state_Share.
			//
		case playerStatus_DidWho:
			ListFriends();
			gPlayerListStatus = playerStatus_Share;
			break;
		
			//
			// Share: Get our sharers & sharees.
			// Goes to state_Info (and thence to state_Idle) after that answer comes in.
			//
		case playerStatus_Share:
			if ( currentTime < gLastAutoCommandTime + kAutoCommandDelay )
				break;
			gLastAutoCommandTime = currentTime;
			
			SendCommand( "be-share", nullptr, true );
#ifdef DEBUG_VERSION
//			ShowMessage( "be-share" );
#endif
			break;
			
			//
			// Info: Received one /be-info.
			// Twiddle thumbs for 3 seconds, then revert back to state_Idle.
			//
		case playerStatus_Info:
			if ( not gLastAutoCommandTime )
				gLastAutoCommandTime = currentTime;
			if ( currentTime < gLastAutoCommandTime + kAutoCommandDelay )
				break;
			gPlayerListStatus = playerStatus_Idle;
			break;
		}
}


/*
**	ProcessPlayerData()
**
**	top-level player list state-machine maintenance:
**	after parsing server responses, go to next state.
**	Now only called in reaction to a be-info, be-who, be-share
*/
static void
ProcessPlayerData( BEPPrefixID id )
{
	switch ( gPlayerListStatus )
		{
		case playerStatus_Idle:
			if ( kBEPPInfoID == id )
				{
				gPlayerListStatus = playerStatus_Info;
				gLastAutoCommandTime = 0;		// wait some in that state?
				}
			break;
		
		case playerStatus_Who:
			if ( kBEPPWhoID == id )
				{
				if ( not gNeedWho )
					gPlayerListStatus = playerStatus_DidWho;
				}
			break;
		
		case playerStatus_Share:
			if ( kBEPPShareID == id )
				gPlayerListStatus = playerStatus_Info;
			break;
		
		case playerStatus_Info:
			// since it's a be-info, we are done
			break;
		}
}

#pragma mark -


/*
**	PlayersWindow::Init()
*/
DTSError
PlayersWindow::Init( uint kind )
{
	DTSError result = DTSWindow::Init( kind );
	if ( noErr != result )
		return result;
	
	pwContent.Attach( this );
	
	// set resize limits
	HISize minSz = CGSizeMake( kMinPlayersWinWidth, kMinPlayersContHeight );
	HISize maxSz = CGSizeMake( kMaxPlayersWinWidth, kMaxPlayersContHeight );
	__Verify_noErr( SetWindowResizeLimits( DTS2Mac( this ), &minSz, &maxSz ) );
	
	// prepare Carbon events
	result = InitTarget( GetWindowEventTarget( DTS2Mac(this) ) );
	if ( noErr != result )
		return result;
	
	const EventTypeSpec evts[] =
		{
			{ kEventClassWindow, kEventWindowDrawContent },
			{ kEventClassWindow, kEventWindowBoundsChanged },
			{ kEventClassWindow, kEventWindowBoundsChanging }
		};
	
	result = AddEventTypes( GetEventTypeCount( evts ), evts );
	
	return result;
}


/*
**	PlayersWindow::Activate()
**
**	check the menu
**	activate the window
*/
void
PlayersWindow::Activate()
{
	DTSMenu * menu = gDTSMenu;
	menu->SetFlags( mPlayersWindow, kMenuChecked );
	
	DTSWindow::Activate();
	
	pwContent.Activate();
	Invalidate();
}


/*
**	PlayersWindow::Deactivate()
**
**	uncheck the menu
**	deactivate the window
*/
void
PlayersWindow::Deactivate()
{
	DTSMenu * menu = gDTSMenu;
	menu->SetFlags( mPlayersWindow, kMenuNormal );
	
	pwContent.Deactivate();
	Invalidate();
	
	DTSWindow::Deactivate();
}


/*
**	PlayersWindow::Size()
**
**	move the future scroll bars
*/
void
PlayersWindow::Size( DTSCoord width, DTSCoord height )
{
	// avoid useless work
	DTSRect bounds;
	GetBounds( &bounds );
	if ( width  == bounds.rectRight - bounds.rectLeft
	&&   height == bounds.rectBottom - bounds.rectTop )
		{
		return;
		}
	
	// call through
	DTSWindow::Size( width, height );
	
	// pin the size
	if ( width < kMinPlayersWinWidth )
		width = kMinPlayersWinWidth;
	
	if ( height < kMinPlayersContHeight )
		height = kMinPlayersContHeight;
	
	SInt32 sbarWd( 15 ), growboxHt( 15 );
	::GetThemeMetric( kThemeMetricScrollBarWidth, &sbarWd );
	::GetThemeMetric( kThemeMetricResizeControlHeight, &growboxHt );
	
	// size the content
	pwContent.Size( width - sbarWd, height );
	
	// place the scroll bar
	gPlayersList.lvScrollBar.Move( width - sbarWd, 0 );
	gPlayersList.lvScrollBar.Size( sbarWd, height - growboxHt );
}


/*
**	PlayersWindow::AdjustCount()
**
**	the number of players has changed, so update the window's zoom size accordingly.
*/
void
PlayersWindow::AdjustCount()
{
	int maxHt = gNumPlayers * gPlayersList.lvCellHeight + kTotalStaticHeights;
	if ( maxHt < kMinPlayersContHeight )
		maxHt = kMinPlayersContHeight;
	else
	if ( maxHt > kMaxPlayersWinHeight )
		maxHt = kMaxPlayersWinHeight;
	
	SetZoomSize( kMaxPlayersWinWidth, maxHt );
}


/*
**	PlayersWindow::KeyStroke()
**
**	bring the game window to the front and give it this keystroke
**	unless it's a list-control key like arrow or page up/down
**	or a menu shortcut key
**	return false if the keystroke should be propagated to the remainder of the app
*/
bool
PlayersWindow::KeyStroke( int ch, uint modifiers )
{
	// cmd-keys go to app for handling
	if ( modifiers & kKeyModMenu )
		return false;
	
	// forward it to the list
	return gPlayersList.KeyStroke( ch, modifiers);
}


/*
**	PlayersWindow::DoWindowEvent()
*/
OSStatus
PlayersWindow::DoWindowEvent( const CarbonEvent& evt )
{
	OSStatus result( eventNotHandledErr );
	OSStatus err( noErr );
	UInt32 attr( 0 );
	HIRect r;
	
	switch ( evt.Kind() )
		{
		case kEventWindowDrawContent:
			DoDraw();
			result = noErr;
			break;
		
		case kEventWindowBoundsChanged:
			{
			evt.GetParameter( kEventParamAttributes, typeUInt32, sizeof attr, &attr );
			if ( attr & kWindowBoundsChangeSizeChanged )
				{
				err = evt.GetParameter( kEventParamCurrentBounds,
						typeHIRect, sizeof r, &r );
				__Check_noErr( err );
				if ( noErr == err )
					{
					StDrawEnv saveEnv( this );
					Size( CGRectGetWidth( r ), CGRectGetHeight( r ) );
					DoDraw();
					result = noErr;
					}
				else
					result = err;
				}
			}
			break;
		
		// "quantize" resizing
		case kEventWindowBoundsChanging:
			{
			const UInt32 kNeedThese = kWindowBoundsChangeSizeChanged
									| kWindowBoundsChangeUserResize;
			err = evt.GetParameter( kEventParamCurrentBounds, typeHIRect, sizeof r, &r );
			if ( noErr == err )
				err = evt.GetParameter( kEventParamAttributes, typeUInt32, sizeof attr, &attr );
			if ( noErr == err
			&&   (attr & kNeedThese) == kNeedThese )
				{
				DTSPoint pt( CGRectGetWidth( r ), CGRectGetHeight( r ) );
				pt = pwContent.BestSize( pt );
				r.size = CGSizeMake( pt.ptH, pt.ptV );
				evt.SetParameter( kEventParamCurrentBounds, typeHIRect, sizeof r, &r );
				result = noErr;
				}
			}
			break;
		}
	
	return result;
}


/*
**	PlayersWindow::HandleEvent()
*/
OSStatus
PlayersWindow::HandleEvent( CarbonEvent& evt )
{
	OSStatus result = eventNotHandledErr;
	
	switch ( evt.Class() )
		{
		case kEventClassWindow:
			result = DoWindowEvent( evt );
			break;
		}
	
	if ( eventNotHandledErr == result )
		result = CarbonEventResponder::HandleEvent( evt );
	
	return result;
}

#pragma mark -


/*
**	PlayersList::DrawNode()
**
**	Draw a single node of the list
*/
void
PlayersList::DrawNode( int inIndex, const DTSRect& inRect, bool bActive )
{
	// theme-font to use for players' names
	const ThemeFontID kFont = kThemeCurrentPortFont;
	
	PlayerNode * player = FindNumInList( inIndex );
	if ( not player )
		return;
	
	bool bSelected = (player == plSelect);
	
	// erase the cell. Special treatment if it's the selected player
	{
	ThemeBrush brush = kThemeBrushListViewBackground;
	if ( bSelected )
		brush = bActive ? kThemeBrushAlternatePrimaryHighlightColor
						: kThemeBrushSecondaryHighlightColor;
	
	__Verify_noErr( SetThemeBackground( brush, 32, true ) );
	Erase( &inRect );
	__Verify_noErr( SetThemeBackground( kThemeBrushListViewBackground, 32, true ) );
	}
	
	// pre-cache some common icons
	const ImageCache * iconFallen 		= CachePicture( 1122 /* 1148 */ );	// 'X/x'
	const ImageCache * iconBard			= CachePicture( kBardPlayerMarkPict );
	
	__Verify_noErr( UseThemeFont( kThemeLabelFont, smRoman ) );
	
	int friends = player->Friendship();
	bool bFallen = (player->pnFlags & kDeadMask) != 0;
	
	DTSRegion saveMask;
	GetMask( &saveMask );
	DTSRect	dst = inRect;
	dst.Inset( 1, 0 );
	SetMask( &dst );
	
	// find an image for this mobile
	if ( const ImageCache * ic = CachePicture( player->pnInfo.mPictID,
									&player->pnPictCacheID,
									player->pnInfo.mColors[0],
									&player->pnInfo.mColors[1] ) )
		{
		Draw1Mobile( ic, dst, bFallen );
		}
	
	// draw the badge of profession
	const ImageCache * todraw = nullptr;
	if ( DTSKeyID iconID = PictureIDForClass( player->pnInfo.mClass & PlayerInfo::class_Mask ) )
		todraw = CachePicture( iconID );
	
	// draw optional right-side icons: fallen, profession, bard...
	if ( bFallen )
		Draw1Icon( iconFallen, dst, 0 );
	
	if ( todraw )
		Draw1Icon( todraw, dst, 12 );
	
	if ( player->pnInfo.mClass & PlayerInfo::class_Bard )
		Draw1Icon( iconBard, dst, 12 );
	
	SetFontStyle( kStyleNormal );
	
	// draw the GM level, if player can see it
	if ( gPlayerIsAGod )
		{
		char tc[2];
		int gmLevel = (player->pnFlags & kGodMask) >> kGodShift;
		*tc = '0' + gmLevel;
		tc[1] = '\0';
		
		// it's the leftmost of all the right-side badges
		if ( gmLevel )
			{
			if ( CFStringRef tc2 = CreateCFString( tc, kCFStringEncodingASCII ) )
				{
				const ThemeFontID kGMInfoFont = kThemeLabelFont;	// note, smaller size
				int w = GetTextBoxWidth( this, tc2, kGMInfoFont ) + 1;
				
				DrawTextBox( this, tc2,
					dst.rectRight - 1, dst.rectTop + 13, kJustRight, kGMInfoFont );
				
				dst.rectRight -= w + 1;
				CFRelease( tc2 );
				}
			}
		}
	
	// setup styles & colors for the player's actual name
	if ( CFStringRef name = CreateCFString( player->pnName ) )
		{
		int style = kStyleNormal;
		if ( player->pnFlags & kSharingMask )	style |= kStyleBold;
		if ( player->pnFlags & kShareeMask )	style |= kStyleUnderline;
		if ( player->pnFlags & kClanMask )		style |= kStyleItalic;
		
		SetFontStyle( style );
		int textWidth = GetTextBoxWidth( this, name, kFont );
		
		// maybe squish long names
		if ( textWidth >= dst.rectRight - dst.rectLeft )
			{
			style |= kStyleCondense;
			SetFontStyle( style );
			}
		
		// finally draw the name
		// use colors for friendship; white for the selected player; black for all others
		DTSColor nameColor( DTSColor::black );
		if ( bSelected && bActive )
			nameColor.SetWhite();
		
		if ( friends > 0 && friends < kFriendFlagCount )
			nameColor = gFriendColors[ friends ];
		
		SetForeColor( &nameColor );
	
		DrawTextBox( this, name, dst.rectLeft, dst.rectTop + 13, kJustLeft, kFont );
		CFRelease( name );
		}
	
	// restore the environment
	SetForeColor( &DTSColor::black );
	SetFontStyle( kStyleNormal );
	SetMask( &saveMask );
}


/*
**	PlayersList::PictureIDForClass()
**
**	return the pictureID to use for a given playerClass
**	or 0 if nothing appropriate
*/
DTSKeyID
PlayersList::PictureIDForClass( int pClass )
{
	DTSKeyID result;
	
	switch ( pClass )
		{
		case  1: result =  635;	break;		// healer -> moonstone
		case  2: result = 1580;	break;		// fighter -> greataxe
		case  3: result =  417;	break;		// full mystic -> sunstone
#ifdef ARINDAL
		case  7: result = 4069;	break;		// game master -> GM icon
#endif
		case 12: result =  624;	break;		// apprentice mystic -> staff
		case 17: result = 1408;	break;		// journeyman mystic -> orb
#ifndef ARINDAL
		case 18: result = 2252;	break;		// bloodmage -> bloodblade
		case 19: result = 1528;	break;		// champion -> fellblade
//		case  5:							// disused old-style "ranger"
		case 20: result =  127;	break;		// ranger -> gossamer
#else
		case 18:
		case 19:
		case 20: result = 1580;	break;		// fighter subclasses -> greataxe
#endif
		default: result = 0;	break;		// unknown
		}
	
	return result;
}

/*
**	PlayersList::Draw1Mobile()
**
**	helper for DrawNode(), to draw the player's mini avatar
*/
void
PlayersList::Draw1Mobile( const ImageCache * icon, DTSRect& dst, bool bFallen )
{
	// the image is 16 poses wide
	int mobileSize = icon->icBox.rectRight >> 4;
	
	DTSRect srcBox;
	int state = bFallen ? kPoseDead : kGetMobilePose( kPoseStand, kFacingSouth );
	srcBox.rectTop	= icon->icBox.rectTop + mobileSize * (state >> 4);
	srcBox.rectLeft = mobileSize * (state & 0x0F);
	if ( icon->icImage.cliPictDef.pdFlags & kPictDefCustomColors )
		++srcBox.rectTop;
	
	srcBox.Size( mobileSize, mobileSize );
	
	DTSRect dstBox( dst.rectLeft, dst.rectTop + 1,
					dst.rectLeft + kMaxPWIconSize, dst.rectTop + 1 + kMaxPWIconSize );
	
	Draw( &icon->icImage.cliImage, kImageTransparentQuality, &srcBox, &dstBox );
	
	dst.rectLeft = dstBox.rectRight + 3;	// update draw rect
}


/*
**	PlayersList::Draw1Icon()
**
**	helper for DrawNode(), to draw the various icons
*/
void
PlayersList::Draw1Icon( const ImageCache * icon, DTSRect& dst, int iconSize )
{
	// paranoia
	if ( not icon )
		return;
	
	DTSRect todrawBounds;
	icon->icImage.cliImage.GetBounds( &todrawBounds );
	DTSRect idst = todrawBounds;
	idst.Offset( -idst.rectLeft, -idst.rectTop );
	
	if ( iconSize )
		idst.Size( iconSize, iconSize );
	else
	if ( idst.rectRight > kMaxPWIconSize || idst.rectBottom > kMaxPWIconSize )
		idst.Size( kMaxPWIconSize, kMaxPWIconSize ); // size is too big. better be square
	
	int w = idst.rectRight + 2;
	idst.Offset( dst.rectRight - w,
				 dst.rectTop + (kPlayerInfoHeight / 2) - (idst.rectBottom / 2) );
	Draw( &icon->icImage.cliImage, kImageTransparentQuality, &todrawBounds, &idst );
	dst.rectRight -= w;
}


/*
**	PlayersList::Click()
**
**	A click that allows dragging across players to change the selection
**	and scroll the list
*/
bool
PlayersList::Click( const DTSPoint& start, ulong time, uint modifiers )
{
	// First, normal click
	ListView::Click( start, time, modifiers );
	
	DTSRect bounds;
	GetBounds( &bounds );
	
	int newSel;
	DTSPoint click;
	while ( IsButtonStillDown() && plSelect )	// TODO: MouseTracker
		{
		// Make the coordinates local
		lvWindow->GetMouseLoc( &click );
		click.Offset( -bounds.rectLeft, -bounds.rectTop );
		
		if ( click.ptV > 0 )
			newSel = lvStartNum + click.ptV / lvCellHeight;
		else
			newSel = -1 + lvStartNum + click.ptV / lvCellHeight;
		
		if ( plSelectNum != newSel )
			OffsetSelectedPlayer( newSel - plSelectNum ); // Handles scrolling
		}
	
	return true;
}


/*
**	PlayersList::ClickNode()
*/
bool
PlayersList::ClickNode( int inIndex, const DTSPoint& /*start*/, ulong time, uint modifiers )
{
	if ( PlayerNode * newPlayer = FindNumInList( inIndex ) )
		{
		HandleNameClick( newPlayer, modifiers, inIndex, time - lvLastClickTime );
		return true;
		}
	
	return false;
}


/*
**	PlayersList::HandleNameClick()
**
**	the click was in a name
*/
void
PlayersList::HandleNameClick( PlayerNode * player, uint mods, int listNum, int deltaTime )
{
	bool selectionChanged = false;
	bool differentPlayer = (player != plSelect);
	bool wasDoubleClick = ( (deltaTime <= (int) ::GetDblTime()) && not differentPlayer );
	bool invalidate = false;
	int friends = player->Friendship();
	
#if ALLOW_MACRO_CLICKS
	// Do a macro unless it is a simple click - which would really screw up the list
	if ( mods )
		{
		if ( DoMacroClick( player->pnName, mods ) )
			return;
		}
#endif  // ALLOW_MACRO_CLICKS
	
	switch ( mods & (kKeyModOption | kKeyModControl | kKeyModShift) )
		{
		// semantics for name-clicking:
		
		// a simple click means toggle selection
		// a double simple-click means... I dunno; "get info", maybe?
		// with the shift key alone, it's the same as no keys
		case 0:
		case kKeyModShift:
			invalidate = true;
			
			if ( differentPlayer )
				{
				// select the newly chosen one
				plSelect = player;
				plSelectNum = listNum;
				StringCopySafe( gSelectedPlayerName, player->pnName, sizeof gSelectedPlayerName );
				selectionChanged = true;
				}
			else
				{
				// it's a click on the currently-selected one
				if ( wasDoubleClick )
					{
					SendCommand( "info", player->pnName, true );
					invalidate = false;
					}
				else
					{
					ClearSelection();
					selectionChanged = true;
					}
				}
			break;
		
		// option means "transfer name to type-in box"
		case kKeyModOption:
			InsertName( player->pnName );
			break;
		
		// control alone means "cycle friendship"
		case kKeyModControl:
			friends = GetNextFriendLabel( friends );
			SetPermFriend( player->pnName, friends );
			player->pnFlags = (player->pnFlags & ~kFriendMask) | (friends << kFriendShift);
			invalidate = true;
			if ( IsPlayerBeingShown( player, -1 ) )
				Update();
			break;
		
		// control-shift means "cycle ignoredness"
		case kKeyModControl | kKeyModShift:
			if ( kFriendBlock == friends )
				friends = kFriendIgnore;
			else
			if ( kFriendIgnore == friends )
				friends = kFriendNone;
			else
				friends = kFriendBlock;
			SetPermFriend( player->pnName, friends );
			player->pnFlags = (player->pnFlags & ~kFriendMask) | (friends << kFriendShift);
			invalidate = true;
			if ( IsPlayerBeingShown( player, -1 ) )
				Update();
			break;
		
		default:
//			Beep();
			break;
		}
	
	if ( selectionChanged )
		UpdateCommandsMenu( gSelectedPlayerName );
	
	if ( invalidate )
		{
//		Update();
		lvWindow->Draw();
		}
}


/*
**	PlayersList::KeyStroke()
*/
bool
PlayersList::KeyStroke( int ch, uint modifiers )
{
	bool result = true;
	
	if ( ListView::KeyStroke( ch, modifiers ) )
		return true;
	
	switch ( ch )
		{
		case kLeftArrowKey:
		case kUpArrowKey:
			OffsetSelectedPlayer( -1 );
			break;
		
		case kRightArrowKey:
		case kDownArrowKey:
			OffsetSelectedPlayer( +1 );
			break;
		
		default:
			// keys that might be type-selection go to the list
			// keys that definitely can't be, are sent to the game window instead
			if ( (modifiers & (kKeyModMenu | kKeyModControl | kKeyModOption))
			||	 not ( strchr( " '-", ch ) || isalnum( ch ) ) )
				{
				if ( DTSWindow * win = gGameWindow )
					{
					win->GoToFront();
					
					// absorb return key by transmuting it into a tab
					if ( kEnterKey == ch
					||	 kReturnKey == ch )
						{
						ch = kTabKey;
						}
					result = win->KeyStroke( ch, modifiers );
					}
				}
			else
				NormalKeyStroke( ch );
			break;
		}
	
	return result;
}


/*
**	PlayersList::NormalKeyStroke()
**
**	attempt type-selection. return true if the selection changed
*/
bool
PlayersList::NormalKeyStroke( int ch )
{
	// when was the last keydown?
	UInt32 now = GetFrameCounter();
	ulong keyInterval = now - plLastKeyTime;
	plLastKeyTime = now;
	
	static int charCount;
	static char typeAhead[ 16 ];
	
	// figure max inter-key delay; see IM:MMT, List Mgr, p. 4-46
	const int kMaxThresh = 120;				// 2 seconds
	int maxThresh = 2 * LMGetKeyThresh();
	if ( maxThresh > kMaxThresh )
		maxThresh = kMaxThresh;
	
	// if old enough, start new typeahead
	if ( keyInterval > (uint) maxThresh )
		charCount = 0;
	
	// don't overflow buffer, though
	else
	if ( charCount > 15 )
		{
		Beep();
		return false;
		}
	
	// save the char (uppercased, ignoring " -'" chars)
	typeAhead[ charCount ] = toupper( ch );
	if ( ch != ' ' && ch != '\'' && ch != '-' )
		++charCount;
	
	// and terminate the string
	typeAhead[ charCount ] = '\0';
	
	// see if we can find this name
	// ( more efficient to cache the last place we looked, but that might vanish)
	const PlayerNode * plyr = gPlayersHead;
	int found = -1;
	for ( ; plyr; plyr = plyr->pnNext )
		{
		char upName[ kMaxNameLen ];
		CompactName( plyr->pnName, upName, sizeof upName );
		found = strncmp( upName, typeAhead, charCount );
		if ( found >= 0 )
			break;
		}
	
	// fail if no such prefix
	if ( not plyr || found != 0 )
		{
		Beep();
		--charCount;
		if ( charCount < 0 )
			charCount = 0;
		return false;
		}
	
	// SelectPlayer() toggles, which is not what I want
	if ( plSelectNum != plyr->pnNum )
		SelectPlayer( plyr->pnName );
	else
	if ( not IsPlayerBeingShown( nullptr, plyr->pnNum ) )
		{
		// scroll selected player into view
		int val = plyr->pnNum - (lvShowNum / 2);
		lvScrollBar.SetValue( val );
		lvStartNum = lvScrollBar.GetValue();
		
		gPlayersWindow->Invalidate();
		}
	
	return true;
}

#pragma mark -


/*
**	PlayersContent::PlayersContent()
**	constructor
*/
PlayersContent::PlayersContent()
	:
	pcPlayersList( &gPlayersList ),
	pcActive( false ),
#ifdef DEBUG_VERSION
		// for regular players, start out by showing killer,
		// not location, since the latter info is comparatively rare nowadays.
		// but do the opposite for debug client
	pcShowLocation( true )
#else
	pcShowLocation( false )
#endif
{
}
	

/*
**	PlayersContent::Size()
**
**	size the content
*/
void
PlayersContent::Size( DTSCoord width, DTSCoord height )
{
	// resize players list
	gPlayersList.Size( width, height - kTotalStaticHeights );
	gPlayersList.Move( 0, kResetAreaHeight );
	
	// Change the height to that actually used
	DTSRect bounds;
	gPlayersList.GetBounds( &bounds );
	height = kTotalStaticHeights + bounds.rectBottom - bounds.rectTop;
	
	pcTopBox.Move( 0, 0 );
	pcTopBox.Size( width, kResetAreaHeight - 2 );
	
	pcBottomBox.Move( 0, bounds.rectBottom + 1 );
	pcBottomBox.Size( width, kFallenInfoHeight + kCountsAreaHeight - 1 );
	
	// inherited
	DTSView::Size( width, height );
}


/*
**	PlayersContent::BestSize()
**	What's the best size for this list?
*/
DTSPoint
PlayersContent::BestSize( DTSPoint pt ) const
{
	int wd = pt.ptH;
	int ht = pt.ptV;
	
	// clamp to limits
	if ( wd < kMinPlayersWinWidth )
		wd = kMinPlayersWinWidth;
	if ( wd > kMaxPlayersWinWidth )
		wd = kMaxPlayersWinWidth;
	
	if ( ht < kMinPlayersContHeight )
		ht = kMinPlayersContHeight;
	if ( ht > kMaxPlayersWinHeight )
		ht = kMaxPlayersWinHeight;
	
	// ensure whole lines
	const int slop = kTotalStaticHeights;
	const int lh = kPlayerInfoHeight;
	
	ht = slop + lh * ((ht - slop) / lh);
	
	return DTSPoint( wd, ht );
}


/*
**	PlayersContent::DoDraw()
**
**	draw the Players content
*/
void
PlayersContent::DoDraw()
{
	const ThemeFontID kFont = kThemeLabelFont;
	
	// erase it all
	DTSRect bounds;
	GetBounds( &bounds );
	Erase( &bounds );
	ThemeDrawState state = pcActive ? kThemeStateActive : kThemeStateInactive;
	
	// Draw the top box
	int width = bounds.rectRight - bounds.rectLeft;
	
	DTSRect b( pcTopBox );
	b.Inset( -1, -1 );
	__Verify_noErr( DrawThemePlacard( DTS2Mac( &b ), state ) );
	
	ThemeTextColor fgColor = pcActive ? kThemeTextColorPlacardActive
									  : kThemeTextColorPlacardInactive;
	__Verify_noErr( SetThemeTextColor( fgColor, 32, true ) );
	// "Double-click here to rebuild the list"
	DrawTextBox( this, CFSTR( _(TXTCL_DBLCLICK_TO_RESET_LIST) ),
		pcTopBox.rectLeft + width/2, pcTopBox.rectTop + 11, kJustCenter, kFont );
	
	// Draw the bottom boxes
	b.Set( pcBottomBox.rectLeft - 1,
		   pcBottomBox.rectTop,
		   pcBottomBox.rectLeft + width + 1,
		   pcBottomBox.rectTop + kCountsAreaHeight + 1 );
	__Verify_noErr( DrawThemePlacard( DTS2Mac( &b ), state ) );
	b.Offset( 0, kCountsAreaHeight );
	__Verify_noErr( DrawThemePlacard( DTS2Mac( &b ), state ) );
	
	// subtract one from the number of shares... don't count self
	int shares = gNumShares - 1;
	if ( shares < 0 )
		shares = 0;
	
	if ( CFStringRef tmpStr = CFStringCreateFormatted( CFSTR( _(TXTCL_PLAYERS_SHARES) ),
			gNumPlayers, shares, gNumSharees ) )
		{
		DrawTextBox( this, tmpStr,
			pcBottomBox.rectLeft + 4, pcBottomBox.rectTop + 11, kJustLeft, kFont );
		CFRelease( tmpStr );
		}
	
	// if selected player is dead, draw his info
	if ( gPlayersList.plSelect && (gPlayersList.plSelect->pnFlags & kDeadMask) )
		{
		CFAbsoluteTime now = CFAbsoluteTimeGetCurrent();
		int time = now - gPlayersList.plSelect->pnDateFell;
		
#ifdef DEBUG_VERSION
		if ( time < 0 || time > 1000000 )
			ShowMessage( "Bad death date player %s", gPlayersList.plSelect->pnName );
#endif
		
		const char * units;
		if ( time < 60 )
			units = _(TXTCL_UNIT_SEC);	// "sec";
		else
		if ( time < 3600 )
			{
			units = _(TXTCL_UNIT_MIN);	// "min";
			time = (time + 30) / 60;
			}
		else
			{
			units = _(TXTCL_UNIT_HOUR);	// "hr";
			time = (time + 1800) / 3600;
			}
		
		// get location of death, or killer's name
		const char * locFell;
		if ( pcShowLocation )
			{
			locFell = gPlayersList.plSelect->pnLocFell;
			if ( '\0' == locFell[0] )
				locFell = _(TXTCL_FALLEN_UNKNOWN_LOCATION);		// "Unknown location"
			}
		else
			{
			locFell = gPlayersList.plSelect->pnKillerName;
			if ( '\0' == locFell[0] )
				locFell = _(TXTCL_FALLEN_BY_UNKNOWN);			// "unknown"
			}
		
		char tmpStr[ 256 ];
		snprintf( tmpStr, sizeof tmpStr, "%s: %d %s", locFell, time, units );
		
		// squish it to fit
		width -= 8;					// allow 4 pixel margins on either side
		
		// we need a mutable string
		if ( CFMutableStringRef tmp = CFStringCreateMutable( kCFAllocatorDefault, sizeof tmpStr ) )
			{
			// do this, rather than CFStringAppendFormat(), because 'locFell' might
			// contain extended MacRoman text, and this call lets you specify the encoding
			CFStringAppendCString( tmp, tmpStr, kCFStringEncodingMacRoman );
			TruncateThemeText( tmp, kFont, state, width - 2, smTruncMiddle, nullptr );
			
			DrawTextBox( this, tmp, b.rectLeft + 5, b.rectTop + 11, kJustLeft, kFont );
			CFRelease( tmp );
			}
		
		}
	
	// Draw subviews
	DTSView::DoDraw();
}


/*
**	PlayersContent::Click()
**
**	Handle clicks in the Player List or on the reset button.
*/
bool
PlayersContent::Click( const DTSPoint& start, ulong time, uint modifiers )
{
	DTSPoint click = start;
	DTSRect bounds;
	GetBounds( &bounds );
	click.Offset( -bounds.rectLeft, -bounds.rectTop );
	
	int deltaTime = time - pcLastClickTime;
	pcLastClickTime = time;
	
	// check for click on reset placard
	if ( click.InRect( &pcTopBox ) )
		{
		if ( deltaTime < int( ::GetDblTime() ) )
			{
			if ( IsKeyDown( kOptionModKey )  &&  IsKeyDown( kMenuModKey ) )
				CompressPlayers();
			else
				ResetPlayers();
			}
		return true;
		}
	
	// and bottom box: toggle where-died vs who-killed mode
	if ( click.InRect( &pcBottomBox ) )
		{
		pcShowLocation = not pcShowLocation;
//		Update();
		gPlayersWindow->Invalidate();
		}
	
	// pass it on to the listview
	return DTSView::Click( start, time, modifiers );
}

#pragma mark -


/*
**	PlayerNode::UpdateFlags()
**
**	Update the player's flags. Track things like god and shares.
*/
bool
PlayerNode::UpdateFlags( uint flags, uint mask )
{
	// do nothing if we already knew the new info
	if ( (pnFlags & mask) == (flags & mask) )
		return false;
	
	if ( flags & kGodMask )
		gPlayerIsAGod = true;
	
	if ( (flags & mask & kSharingMask) != (pnFlags & mask & kSharingMask) )
		{
		if (flags & mask & kSharingMask)
			++gNumShares;
		else
			--gNumShares;
		}
	
	if ( (flags & mask & kShareeMask) != (pnFlags & mask & kShareeMask) )
		{
		if ( flags & mask & kShareeMask )
			++gNumSharees;		// Perhaps notify if sharees < 5
		else
			--gNumSharees;
		}
	
	// Update the flags
	pnFlags = (pnFlags & ~mask) | (flags & mask);
	
	return true;
}


/*
**	ClearSelection()
**
**	Clear the current selection from the three places it is stored.
*/
void
ClearSelection()
{
	gPlayersList.plSelect = nullptr;
	gPlayersList.plSelectNum = -1;
	gSelectedPlayerName[ 0 ] = '\0';
}


/*
**	FriendsList::FriendsList()
**	constructor
*/
FriendsList::FriendsList( const PlayerNode * plyr )
	: mPlayer( plyr )
{
}


/*
**	FriendsList::AddFriend()		[ static ]
**
**	during the startup loading of the playerlist via /BE-WHO, we
**	encountered a friend. Add him to a list, so we can print
**	all online friends together, once, when the be-who scan is finished
*/
void
FriendsList::AddFriend( const PlayerNode * inFriend )
{
	FriendsList * fl = NEW_TAG( "FriendsList" ) FriendsList( inFriend );
	if ( fl )
		fl->InstallFirst( sFriendsListRoot );
}


/*
**	FriendsList::PrintFriends()		[ static ]
**
**	once the initial /BE-WHO scan is completed, we can
**	print out the whole list in one fell swoop.
**	Since the only time when friends are added to the list
**	is during the initial scan, we know the list will only get
**	printed once per log-on (at most).
**	Use label <=0 to show entire list.
*/
void
FriendsList::PrintFriends( int label )
{
	// alphabetize
	ReverseLinkedList( sFriendsListRoot );
	
	// accumulate the names
	SafeString msg;
	
	if ( label <= 0 )
		msg.Format( "%s", _(TXTCL_FRIENDS_ONLINE) );	// "Friends online: "
	else
	if ( kFriendBlock == label )
		msg.Format( "%s", _(TXTCL_BLOCKED_ONLINE) );	// "Blocked players online: "
	else
	if ( kFriendIgnore == label )
		msg.Format( "%s", _(TXTCL_IGNORED_ONLINE) );	// "Ignored players online: "
	else
		{
		// "Friends labelled '%s' online: "
		msg.Format( _(TXTCL_FRIENDS_LABELLED_ONLINE), sFriendLabelNames[ label ] );
		}
	
	bool bNeedsComma = false;
	for ( const FriendsList * fl = sFriendsListRoot;  fl ;  fl = fl->linkNext )
		{
		if ( const PlayerNode * pn = fl->mPlayer )
			{
			if ( bNeedsComma )
				msg.Append( ", " );
			bNeedsComma = true;
			
			// add one name
			msg.Append( pn->pnName );
			}
		}
	
	// if we added any names at all, the bool will be set
	if ( bNeedsComma )
		msg.Append( "." );
	else
		msg.Append( _(TXTCL_ONLINE_NONE) );		// "none."
	
	ShowInfoText( msg.Get() );
	
	// reclaim the memory used for the list and all its nodes.
	FriendsList::DeleteLinkedList( sFriendsListRoot );
	sFriendsListRoot = nullptr;
}


/*
**	ListFriends()
**	uses the FriendsList class machinery to implement the /WHOLABEL <label> command
*/
void
ListFriends( int label /* = -1 */ )
{
	for ( const PlayerNode * node = gPlayersHead; node; node = node->pnNext )
		{
		if ( ( label <= 0 && node->IsFriendly() )
		||   label == node->Friendship() )
			{
			FriendsList::AddFriend( node );
			}
		}
	FriendsList::PrintFriends( label );
}

#pragma mark -


/*
**	NameComparison()
**
**	replacement for strcmp(3) that ignores case and non-alphas
*/
static int
NameComparison( const char * name1, const char * name2 )
{
	char cname1[ kMaxNameLen ];
	char cname2[ kMaxNameLen ];
	CompactName( name1, cname1, kMaxNameLen );
	CompactName( name2, cname2, kMaxNameLen );
	return strcmp( cname1, cname2 );
}


/*
**	NameComparisonFixed()
**
**	just like NameComparison(), but assumes name1 is already compacted
**	saves some time
*/
static int
NameComparisonFixed( const char * name1, const char * name2 )
{
	char cname2[ kMaxNameLen ];
	CompactName( name2, cname2, kMaxNameLen );
	return strcmp( name1, cname2 );
}


/*
**	SplayPlayer()
**
**	This is the heart of the self balancing tree.
**	Basically it finds the node for the player
**	and then shuffles the tree to make that the root node.
*/
static PlayerNode *
SplayPlayer( PlayerNode * tree, const char * name )
{
	if ( not tree )
		return nullptr;
	
	PlayerNode		next("");
	PlayerNode * 	left = &next;
	PlayerNode * 	right = &next;
	PlayerNode * 	tmp;
	
	char cname[ kMaxNameLen ];
	CompactName( name, cname, sizeof cname );
	
	while ( true )
		{
		int dir = NameComparisonFixed( cname, tree->pnName );
		if ( dir < 0 )
			{
			if ( not tree->pnLeft )
				break;
			if ( NameComparisonFixed( cname, tree->pnLeft->pnName ) < 0 )
				{
				tmp = tree->pnLeft;
				tree->pnLeft = tmp->pnRight;
				tmp->pnRight = tree;
				tree = tmp;
				if ( not tree->pnLeft )
					break;
				}
			right->pnLeft = tree;
			right = tree;
			tree = tree->pnLeft;
			}
		else
		if ( dir > 0 )
			{
			if ( not tree->pnRight )
				break;
			if ( NameComparisonFixed( cname, tree->pnRight->pnName ) > 0 )
				{
				tmp = tree->pnRight;
				tree->pnRight = tmp->pnLeft;
				tmp->pnLeft = tree;
				tree = tmp;
				if ( not tree->pnRight )
					break;
				}
			left->pnRight = tree;
			left = tree;
			tree = tree->pnRight;
			}
		else
			break;
		
		dir = NameComparisonFixed( cname, tree->pnName );
		if ( dir < 0 )
			{
			if ( not tree->pnLeft )
				break;
			right->pnLeft = tree;
			right = tree;
			tree = tree->pnLeft;
			}
		else
		if ( dir > 0 )
			{
			if ( not tree->pnRight )
				break;
			left->pnRight = tree;
			left = tree;
			tree = tree->pnRight;
			}
		else
			break;
		
		dir = NameComparisonFixed( cname, tree->pnName );
		if ( dir < 0 )
			{
			if ( not tree->pnLeft )
				break;
			right->pnLeft = tree;
			right = tree;
			tree = tree->pnLeft;
			}
		else
		if ( dir > 0 )
			{
			if ( not tree->pnRight )
				break;
			left->pnRight = tree;
			left = tree;
			tree = tree->pnRight;
			}
		else
			break;
		
		dir = NameComparisonFixed( cname, tree->pnName );
		if ( dir < 0 )
			{
			if ( not tree->pnLeft )
				break;
			right->pnLeft = tree;
			right = tree;
			tree = tree->pnLeft;
			}
		else
		if ( dir > 0 )
			{
			if ( not tree->pnRight )
				break;
			left->pnRight = tree;
			left = tree;
			tree = tree->pnRight;
			}
		else
			break;
		}
	
	left->pnRight = tree->pnLeft;
	right->pnLeft = tree->pnRight;
	tree->pnLeft = next.pnRight;
	tree->pnRight = next.pnLeft;
	
	return tree;
}


/*
**	UpdatePlayer()
**
**	Look up the player. If they don't exist, create them if requested.
**	Then update their flags. Return the player node if requested.
*/
static bool
UpdatePlayer(	const char * name,
				uint flags,
				uint mask,
				PlayerNode ** player,
				bool createNewPlayer,
				bool updateList = true )
{
	PlayersWindow *	win = static_cast<PlayersWindow *>( gPlayersWindow );
	bool changed = false;
	bool updateScrollBar = false;
	int cnt;
	
	// reject impossible names
	for ( cnt = 0; name[ cnt ] && cnt < kMaxNameLen; ++cnt )
		{
		int tc = ((const uchar *) name)[ cnt ];
		
#ifdef DEBUG_VERSION
		// (nearly) all names are valid in GM client, because you or your GM buddies
		// may have adopted arbitrarily weird names
		if ( tc >= ' ' )
			continue;
#endif
		if ( isalpha( tc ) )
			continue;
		
		if ( strchr( " .-'[]", tc ) )
			continue;
		
		return false;
		}
	
	if ( cnt < 3 )
		return false;
	
	// look up the name
	gPlayersRoot = SplayPlayer( gPlayersRoot, name );
	
	char fixedName[ kMaxNameLen ];
	CompactName( name, fixedName, sizeof fixedName );
	
	if ( not gPlayersRoot
	||	 0 != NameComparisonFixed( fixedName, gPlayersRoot->pnName ) )
		{
		// not found in tree
		PlayerNode * tmp = nullptr;
		
		//
		// if we have a saved list, let's look into it..
		//
		if ( gPlayerSaveHead )
			{
			UInt32 time;
			MyGetDateTime( &time );
			//
			// If it's too old, clear it
			//
			if ( time > gPlayerSaveTime + kSavedPlayersStaleDelay )
				{
				// "* Purged saved player list"
				ShowMessage( _(TXTCL_PURGED_SAVED_PLAYER_LIST) );
				RemoveAllPlayersFrom( &gPlayerSaveHead );
				gPlayerSaveTime = 0;
				}
			else
				{
				PlayerNode * n = gPlayerSaveHead;
				for ( ; n; n = n->pnNext )
					if ( not NameComparisonFixed( fixedName, n->pnName ) )
						break;
				if ( n )
					{
					if ( n == gPlayerSaveHead )
						gPlayerSaveHead = n->pnNext;
					
					// detach it
					if ( n->pnNext )
						n->pnNext->pnPrev = n->pnPrev;
					if ( n->pnPrev )
						n->pnPrev->pnNext = n->pnNext;
					n->pnNext = n->pnPrev = n->pnLeft = n->pnRight = nullptr;
					tmp = n;
					
					// we have to turn off their BEWho bit
					// or they'll cause the next round of /be-whos to exit too soon
					tmp->pnFlags &= ~kBEWhoMask;
					
//					ShowMessage( BULLET " Revived %s", tmp->pnName );
					}
				}
			}
		
		if ( not tmp )
			{
			if ( not createNewPlayer )
				return false;
			
			tmp = NEW_TAG("PlayerNode") PlayerNode( name );
			if ( not tmp )
				{
				// "Failed to allocate memory for a player."
				ShowMessage( _(TXTCL_FAILED_ALLOCATE_PLAYER) );
				return false;
				}
			}
		
		// reset shares, friends etc if character has changed from saved list
		tmp->pnFlags &= ~(kSharingMask | kFriendMask | kClanMask | kShareeMask);
		if ( PlayerInfo::LoadPlayer( tmp ) )
			{
			// set "same-clan" bit if they are the same as me,
			// and we're not both in clan #0.
			if ( tmp->pnInfo.mClan && gThisPlayer )
				{
				const PlayerNode * meInfo = gThisPlayer->descPlayerRef;
				if ( meInfo
				&&	 meInfo->pnInfo.mClan == tmp->pnInfo.mClan )
					{
					tmp->pnFlags |= kClanMask;
					}
				}
			}
		
		int friends = GetPermFriend( name );
		if ( friends )
			tmp->pnFlags |= (friends << kFriendShift);
		
		++gNumPlayers;
		gPlayersList.lvNumNodes = gNumPlayers;
		updateScrollBar = true;
		
		changed = true;
		
		if ( not gPlayersRoot )
			{
			tmp->pnNum = 0;
			gPlayersRoot = tmp;
			gPlayersHead = tmp;
			gPlayersTail = tmp;
			}
		else
			{
			// insert into tree
			if ( NameComparison( name, gPlayersRoot->pnName ) < 0 )
				{
				tmp->pnLeft = gPlayersRoot->pnLeft;
				tmp->pnRight = gPlayersRoot;
				gPlayersRoot->pnLeft = nullptr;
				
				tmp->pnNext = gPlayersRoot;
				tmp->pnPrev = gPlayersRoot->pnPrev;
				if ( tmp->pnPrev == nullptr )
					gPlayersHead = tmp;
				else
					tmp->pnPrev->pnNext = tmp;
				gPlayersRoot->pnPrev = tmp;
				}
			else
				{
				tmp->pnRight = gPlayersRoot->pnRight;
				tmp->pnLeft = gPlayersRoot;
				gPlayersRoot->pnRight = nullptr;
				
				tmp->pnPrev = gPlayersRoot;
				tmp->pnNext = gPlayersRoot->pnNext;
				if ( tmp->pnNext == nullptr )
					gPlayersTail = tmp;
				else
					tmp->pnNext->pnPrev = tmp;
				gPlayersRoot->pnNext = tmp;
				}
			gPlayersRoot = tmp;
			
			tmp = tmp->pnNext;
			
			if ( not tmp )
				gPlayersRoot->pnNum = gNumPlayers - 1;
			else
				gPlayersRoot->pnNum = tmp->pnNum;
			
			while ( tmp )
				{
				++tmp->pnNum;
				tmp = tmp->pnNext;
				}
			}
		
		// keep the window content up-to-date
		if ( gPlayersRoot->pnNum <= gPlayersList.lvStartNum )
			++gPlayersList.lvStartNum;
		
		if ( gPlayersList.plSelect
		&&	 gPlayersRoot->pnNum <= gPlayersList.plSelectNum )
			{
			++gPlayersList.plSelectNum;
			}
		}
	
	changed |= gPlayersRoot->UpdateFlags( flags, mask );
	
	if ( player )
		*player = gPlayersRoot;
	
	if ( updateList )
		{
		// A player node changed
		if ( changed )
			win->Invalidate();
		
		// the number of players changed
		if ( updateScrollBar )
			{
			gPlayersList.AdjustRange();
			win->AdjustCount();
			}
		}
	
	return changed;
}


/*
**	RemovePlayer()
**
**	Remove the player from the tree and make everything right.
*/
static bool
RemovePlayer( const char * name )
{
	PlayersWindow *	win = static_cast<PlayersWindow *>( gPlayersWindow );
	
	if ( not win )
		return false;
	
	if ( not gPlayersRoot )
		return false;
	
	gPlayersRoot = SplayPlayer( gPlayersRoot, name );
	if ( NameComparison( name, gPlayersRoot->pnName ) != 0 )
		return false;
	
	// Remove it from the tree;
	PlayerNode * tmp;
	if ( not gPlayersRoot->pnLeft )
		tmp = gPlayersRoot->pnRight;
	else
		{
		tmp = SplayPlayer( gPlayersRoot->pnLeft, name );
		tmp->pnRight = gPlayersRoot->pnRight;
		}
	
	// Remove it from the list.
	if ( not gPlayersRoot->pnPrev )
		{
		gPlayersHead = gPlayersRoot->pnNext;
		if ( gPlayersHead )
			gPlayersHead->pnPrev = nullptr;
		}
	else
		gPlayersRoot->pnPrev->pnNext = gPlayersRoot->pnNext;
	
	if ( not gPlayersRoot->pnNext )
		{
		gPlayersTail = gPlayersRoot->pnPrev;
		if ( gPlayersTail )
			gPlayersTail->pnNext = nullptr;
		}
	else
		gPlayersRoot->pnNext->pnPrev = gPlayersRoot->pnPrev;
	
	for ( PlayerNode * tmp2 = gPlayersRoot->pnNext; tmp2; tmp2 = tmp2->pnNext )
		--tmp2->pnNum;
	
	int num = gPlayersRoot->pnNum;
	
	gPlayersRoot->UpdateFlags( 0, ~0 );
	
	// save the players, so its date is updated too
	PlayerInfo::SavePlayer( gPlayersRoot );
	
	delete gPlayersRoot;
	gPlayersRoot = tmp;
	
	--gNumPlayers;
	
	if ( num <= gPlayersList.lvStartNum )
		--gPlayersList.lvStartNum;
	
	if ( gPlayersList.plSelect )
		{
		if ( num < gPlayersList.plSelectNum )
			--gPlayersList.plSelectNum;
		else
		if ( num == gPlayersList.plSelectNum )
			ClearSelection();
		}
	gPlayersList.lvNumNodes = gNumPlayers;
	
	// adjust range first, in case it was player #1 that got removed
	gPlayersList.AdjustRange();
	win->Invalidate();
	win->AdjustCount();
	
	UpdateCommandsMenu( gSelectedPlayerName );
	return IsPlayerBeingShown( nullptr, num );
}


//
// Delete the specified queue
//
void
RemoveAllPlayersFrom( PlayerNode ** queue )
{
	PlayerNode * cur = *queue;
	while ( cur )
		{
		PlayerNode * tmp = cur;
		
		// save all the players, so their dates are updated too
		PlayerInfo::SavePlayer( cur );
		
		cur = cur->pnNext;
		delete tmp;
		}
	*queue = nullptr;
}


#if 0  // -- unused
/*
**	RemoveAllPlayers
**
**	Delete all the players and reset some other state.
*/
static void
RemoveAllPlayers()
{
	RemoveAllPlayersFrom( &gPlayersHead );
	
	gPlayersHead = nullptr;
	gPlayersTail = nullptr;
	gPlayersRoot = nullptr;
	gNumPlayers = 0;
	gPlayersList.lvNumNodes = gNumPlayers;
	
	PlayersWindow * win = static_cast<PlayersWindow *>( gPlayersWindow );
	gPlayersList.AdjustRange();
	win->AdjustCount();
}
#endif  // 0


/*
**	IsPlayerBeingShown()
**
**	Figure out if a player is currently on screen in the Players List.
**	If player is NULL then num must be valid.
**	Or rather, if num is -1, then player must be valid.
*/
bool
IsPlayerBeingShown( PlayerNode * player, int num )
{
	// assume lookup by num
	int cnt = num;
	
	if ( -1 == num )
		{
		// do lookup by player
		for ( const PlayerNode * pList = gPlayersHead; pList; pList = pList->pnNext )
			{
			++cnt;
			if ( pList == player )
				break;
			}
		}
	
	// is it visible?
	if ( cnt >= gPlayersList.lvStartNum
	&&	 cnt <  gPlayersList.lvStartNum + gPlayersList.lvShowNum )
		{
		return true;
		}
	return false;
}


/*
**	FindNumInList()
**
**	Walk through the tree to find the nth player.
*/
PlayerNode *
FindNumInList( int num )
{
	PlayerNode * player = gPlayersRoot;
	while ( player )
		{
		if ( num == player->pnNum )
			return player;
		
		if ( num < player->pnNum )
			player = player->pnLeft;
		else
			player = player->pnRight;
		}
	
	return nullptr;
}


/*
**	LookUpPlayer()
**
**	Find the player node, validate the cached reference and return the node.
*/
static PlayerNode *
LookUpPlayer( const DescTable * desc )
{
	PlayerNode * player = nullptr;
	if ( not desc )
		return nullptr;
	
	if ( desc->descType != kDescPlayer
	&&	 desc->descType != kDescThoughtBubble )
		{
		return nullptr;
		}
	
	player = desc->descPlayerRef;
	if ( not player )
		{
		bool createNewPlayer = (kDescPlayer == desc->descType);
		UpdatePlayer( desc->descName, 0, 0, &player, createNewPlayer );
		
		if ( not player )
			{
			desc->descPlayerRef = nullptr;
			return nullptr;
			}
		
		// did the player change anything we tend to keep track of locally?
		
		// (NB: we don't save the pictures of players who're in a boat.
		// elsewhere we also discount players wearing the 2002/2004 April Fool Nethack "@" icon.
		// We maybe also should not track pictures of morphed rangers, Halloween costumes,
		// butterflies, undead costumes from undine cavern, etc. But that would require...
		// 1. Socks' "picture_priority.h" machinery would need to implement some kind of
		//		"non-client-cached" status flag; and
		// 2. It would have to be able to convey that status to the server; and
		// 3. The server, in turn, would have to transmit that status flag to the client
		//		in the descriptor data.
		// [in other words: Not gonna happen anytime soon.])
		
		if ( (   player->pnInfo.mPictID != desc->descID
			 ||	 player->pnInfo.mColors[0] != desc->descNumColors
			 ||	 memcmp( &player->pnInfo.mColors[1], desc->descColors, desc->descNumColors) )
		&&	 desc->descID != kBoatPlayerPict )
			{
				player->pnInfo.mPictID		= desc->descID;
				player->pnPictCacheID		= desc->descCacheID;
				player->pnInfo.mColors[0]	= desc->descNumColors;
				memcpy( &player->pnInfo.mColors[1], desc->descColors, desc->descNumColors );
				PlayerInfo::SavePlayer( player );
				
				PlayersWindow * win = static_cast<PlayersWindow *>( gPlayersWindow );
				if ( win
				&&	 IsPlayerBeingShown( player, -1 ) )
					{
					win->Invalidate();
					}
			}
		
		desc->descPlayerRef = player;
		}
	
	return player;
}

#pragma mark -


/*
**	ExtractTaggedText()
**
**	if text contains a sequence <anything-1>'tag'<any-2>'tag'<any-3>
**	then extract <any-2> and return pointer to any-3. else return NULL.
*/
static const char *
ExtractTaggedText(	const char * text,
					const char * tag,
					size_t taglen,
					char * match,
					size_t maxlen )
{
	// find the first tag
	const char * pos = strstr( text, tag );
	if ( not pos )
		return nullptr;
	
	// find the closing tag
	const char * pos2 = strstr( pos + taglen, tag );
	if ( not pos2 )
		return nullptr;
	
	// don't overflow
	size_t matchlen = pos2 - pos - taglen;
	if ( matchlen >= maxlen )
		return nullptr;
	
	// extract it
	BufferToStringSafe( match, maxlen, pos + taglen, matchlen );
//	strncpy( match, pos + taglen, matchlen );
	match[ matchlen ] = 0;
	
	return pos2 + taglen;
}


// some convenient shorthands for the above:
// ExtractPlayer() only looks at kBEPPPlayerName tags, and assumes a kMaxNameLen buffer
inline static const char *
ExtractPlayer( const char * text, char * name )
{
	return ExtractTaggedText( text, kBEPPPlayerName, sizeof(kBEPPPlayerName) - 1,
		name, kMaxNameLen );
}


// Extract doesn't assume anything, but obviates one semi-redundant argument
inline static const char *
Extract( const char * text, const char * tag, char * name, size_t size )
{
	return ExtractTaggedText( text, tag, strlen(tag), name, size );
}


/*
**	TestTextForWho()
**
**	Parse the text for a \who response.
**	Update each player name with their GM status.
**	Make a note of how many players for the auto-caching function.
*/
static bool
TestTextForWho( const char * text )
{
	PlayersWindow * win = static_cast<PlayersWindow *>( gPlayersWindow );
	
	const char WHO_STR[] = "In the world are ";
	const char ONLY_ONE_STR[] = "You are the only one in the lands.";
	
	const size_t whoLen = sizeof WHO_STR - 1;
	const size_t onlyOneLen = sizeof ONLY_ONE_STR - 1;
	
	size_t end = whoLen;
	char name[ kMaxNameLen ];
	
	// handle empty world
	if ( 0 == strncmp( text, ONLY_ONE_STR, onlyOneLen ) )
		return true;
	
	// not a /who response at all?
	if ( 0 != strncmp( text, WHO_STR, whoLen ) )
		return false;
	
	while ( text[end] != ':' )
		++end;
	
	const char * loc = text + end;
	for (;;)
		{
		uint flags = 0;
		loc = ExtractPlayer( loc, name );
		if ( not loc )
			break;
		
		// /WHO format now optionally includes GM-lvl in parens
		if ( '(' == *loc )
			{
			while ( ')' != *loc )
				++loc;
			int gmLevel = *(loc - 1) - '0';
			flags = (gmLevel << kGodShift) & kGodMask;
			}
		
		// don't redraw now...
		UpdatePlayer( name, flags, kGodMask, nullptr, true, false );
		}
	
	// ... rather, wait until after putting all new ones in the list
	win->Invalidate();
	
	gPlayersList.AdjustRange();
	win->AdjustCount();
	
	return true;
}


/*
**	TestTextForShare()
**
**	Parse the text for a \share response.
**	Update each player's share status.
**	returns: 0 if not a share message at all;
**			 1 if a share message but we ignore it;
**			 2 if a share message we care about.
*/
static int
TestTextForShare( const char * text )
{
#define	SHARE_STR		"Currently sharing their experiences with you"
#define YOU_NOT_SHARE	"You are not sharing experiences with anyone."
#define YOU_ARE_SHARE	"You are sharing experiences with "
#define YOU_BEG_SHARE	"You begin sharing your experiences with "
#define YOU_NO_SHARE	"You are no longer sharing experiences with "
#define YOU_UNSHARE		"You are no longer sharing experiences with anyone."
	
	const size_t shareLen			= sizeof SHARE_STR		- 1;
	const size_t youNotShareLen		= sizeof YOU_NOT_SHARE	- 1;
	const size_t youAreShareLen		= sizeof YOU_ARE_SHARE	- 1;
	const size_t youBegShareLen		= sizeof YOU_BEG_SHARE	- 1;
	const size_t youStopShareLen	= sizeof YOU_NO_SHARE	- 1;
	const size_t youUnshareLen		= sizeof YOU_UNSHARE	- 1;
	
	const char * pos;
	char name[ kMaxNameLen ];
	
	if ( 0 == strncmp( YOU_NOT_SHARE, text, youNotShareLen )
	||	 0 == strncmp( YOU_UNSHARE, text, youUnshareLen ) )
		{
		// sharing with no one
		ResetSharees();
		return 1;
		}
	
	// handle the case of unsharing with your only sharee
	// (server doesn't send a followup 'You_Not_Share")
	if ( 1 == gNumSharees
	&&	 0 == strncmp( YOU_NO_SHARE, text, youStopShareLen ) )
		{
		pos = ExtractPlayer( text, name );
		if ( pos )
			UpdatePlayer( name, 0, kShareeMask, nullptr, true, false );
		return 1;
		}
	
	if ( 0 == strncmp( YOU_BEG_SHARE, text, youBegShareLen ) )
		{
		// only sharing with one person
		pos = ExtractPlayer( text, name );
		if ( pos )
			UpdatePlayer( name, kShareeMask, kShareeMask, nullptr, true, false );
		return 1;
		}
	
	if ( 0 == strncmp( YOU_ARE_SHARE, text, youAreShareLen ) )
		{
		// Full sharee list
		pos = text + youAreShareLen;
		ResetSharees();
		
		while ( pos )
			{
			// pos points near the start of the next unprocessed name
			// (specifically, past the _prior_ name's trailing <pn> tag)
			pos = ExtractPlayer( pos, name );
			if ( pos )
				UpdatePlayer( name, kShareeMask, kShareeMask, nullptr, true, false );
			}
		return 1;
		}
	
	// OK, we've handled sharees (downstream). Now let's do sharers (upstream)
	if ( 0 != strncmp( SHARE_STR, text, shareLen ) )
		return 0;
	
	pos = text + shareLen;
	while ( pos )
		{
		// pos points at a ':' or a ',' so move it forward 2 places.
		pos = ExtractPlayer( pos, name );
		if ( pos )
			UpdatePlayer( name, kSharingMask, kSharingMask, nullptr, true, false );
		}
	
	return 2;
}


/*
**	TestForDeathText()
**
**	Parse the text for a has-fallen message. Extract the name of the fallen player,
**	the monster (or player) who did the deed, and the location thereof.
**	Assumes the destination string buffer are of appropriate size (256 for 'where').
*/
static bool
TestForDeathText( const char * text, char * name, char * monster, char * where )
{
	// pessimism: assume we can't extract any info
	name[0] =
	monster[0] =
	where[0] = '\0';
	
	// find name of fallen player
	const char * gotname = ExtractPlayer( text, name );
	if ( not gotname )
		return false;
	
	// find name of killer (monster, or perhaps another player if PVP)
	const char * gotkiller = Extract( text, kBEPPMonsterName, monster, kMaxNameLen );
	if ( not gotkiller )
		ExtractPlayer( gotname, monster );
	
	// find area name (mostly obsolete now, except for GMs)
	Extract( text, kBEPPLocation, where, 256 );
	
#ifdef DEBUG_VERSION
//	ShowMessage( BULLET " Death: %s to: %s, at: %s", name, monster, where );
#endif
	
	return true;
}


/*
	(yet another) new deal:
	now, when we get an info packet, of any kind, we just turn around and
	ask for a be-info packet on that same person. We don't have to parse
	the text info; all that is concentrated in the be-info handler
*/

/*
**	TestTextForBard()
**
**	scan for bard-related info; extract player name and bard-level, if present.
*/
static bool
TestTextForBard( const char * text, char * name, int * bard )
{
	// we're looking for things like:	(the '*' are actually 0xA5 bullets)
	//		* Pachook is a Bard Quester
	//		* Diotima is not a Bard
	
	const char * const levels[] =
		{
		TXTCL_IS_NOT_BARD,		// " is not a Bard",
		TXTCL_IS_BARD_GUEST,	// " is a Bard Guest",
		TXTCL_IS_BARD_QUESTER,	// " is a Bard Quester",
		TXTCL_IS_BARD,			// " is a Bard",
		TXTCL_IS_BARD_TRUSTEE,	// " is a Bard Trustee",
		TXTCL_IS_BARD_MASTER,	// " is a Bard Master",
		TXTCL_IS_BARD_CRAFTER,	// " is a Bard Crafter",
		nullptr
		};
	
	const char * current = text;
	
	if ( strncmp( current, BULLET " ", 2 ) )
		return false;
	current += 2;
	
	const char * end = strchr( current, '.' );
	if ( not end )
		end = current + strlen( current );
	const char * lev = nullptr;
	int bardLevel = -1;
	
	for ( int i = 0; levels[i]; ++i )
		{
		lev = end - strlen( levels[i] );
		if ( lev > current )
			{
			if ( not strncmp( lev, levels[i], strlen(levels[i]) ) )
				{
				bardLevel = i;
				break;
				}
			}
		}
	if ( -1 == bardLevel )
		return false;
	
	if ( lev - current >= kMaxNameLen )
		return false;
	
	// extract the name, for a starter
	if ( not ExtractPlayer( current, name ) )
		return false;
	
	*bard = bardLevel;
	return true;
}


/*
**	ParseBackEndInfo()
**
**	decipher a /be-info command
**	returns true to suppress display
*/
static bool
ParseBackEndInfo( const char * text )
{
	// "-be" "-in" "error\t\r"		(please read '-' as the BEPP character, opt-l)
	// "-be" "-in" "error\t<name>\r"
	// "-be" "-in" "-pn" <name> "-pn" "\t" followed by
	//		race, gender, class, (#) clan, itmRight, itmLeft, description, level ( == -1)
	//		separated by tabs
	
	// we have already seen the "-be" and the "-in"
	
	char name[ kMaxNameLen ];
	
	if ( kBEPPChar != *text )
		{
		// was error...
		const char BEInfoErr[] = "error\t";
		
		// but was it an "error" error?
		if ( 0 != strncmp( text, BEInfoErr, sizeof BEInfoErr - 1 ) )
			return false;
		
		// maybe we auto-info'ed a player who just left the game: so remove the entry.
		const char * namestart = text + sizeof BEInfoErr - 1;
		if ( '\0' != *namestart )
			RemovePlayer( namestart );
		
		// suppress it
		return true;
		}
	
	const char * current = ExtractPlayer( text, name );
	if ( not current )	// name overflow?
		return true;
	
#ifdef DEBUG_VERSION
//	ShowMessage( "Got be-info for '%s'", name );
#endif
	
	while ( '\t' == *current )
		++current;
	
	uint race = 0;
	for ( int i = 0; kPlayerRaces[i]; ++i )
		{
		// ignore "hidden" races, whose names are empty in the race table
		if ( *kPlayerRaces[i]
		&&   not strncmp( current, kPlayerRaces[i], strlen(kPlayerRaces[i] ) ) )
			{
			race = i;
			current += strlen( kPlayerRaces[i] );
			break;
			}
		}
	
	while ( '\t' != *current )
		++current;
	++current;
	
	uint gender = 0;
	for ( int i = 0;  kPlayerGenders[i];  ++i )
		{
		// ignore "hidden" genders too (not that there are any...)
		if ( *kPlayerGenders[i]
		&&   not strncmp( current, kPlayerGenders[i], strlen(kPlayerGenders[i] ) ) )
			{
			gender = i;
			current += strlen( kPlayerGenders[i] );
			break;
			}
		}
	
	while ( '\t' != *current )
		++current;
	++current;
	
	uint classe = 0;
	for ( int i = 0; kPlayerClass[i]; ++i )
		{
		// ditto "hidden" classes
		if ( *kPlayerClass[i]
		&&   not strncmp( current, kPlayerClass[i], strlen(kPlayerClass[i] ) ) )
			{
			classe = i;
			current += strlen( kPlayerClass[i] );
			break;
			}
		}
	
	while ( '\t' != *current )
		++current;
	++current;
	
	// v141+: server sends '(' <clan num> ') ' <clan name>
	uint clan = 0;
	
	// have we got the open-paren?
	if ( '(' == *current
	&&	 isdigit( current[1] ) )
		{
		// yes, it's a post 141.1 server
		++current;
		
		// get the number
		clan = atoi( current );
		
		// skip the close-paren and the space
		while ( ')' != *current )
			++current;
		while ( ' ' != *current )
			++current;
		++current;
		}
	
	const char * clanEnd = current;
	while ( '\t' != *clanEnd )
		++clanEnd;
	
	char clanName[ 256 ];
	BufferToStringSafe( clanName, sizeof clanName, current, clanEnd - current );
	
	// the rest we don't care about
	
	// write it to the file...
	PlayerNode * player = nullptr;
	UpdatePlayer( name, 0, 0, &player, false, false );
	if ( player )
		{
		++race;	// to know if we already have it, even raceless
			// in other words, we want to distinguish "this player is known to be raceless"
			// from "we have no idea at all about their race"
		
		if ( race	!= player->pnInfo.mRace
		||	 classe	!= (player->pnInfo.mClass & PlayerInfo::class_Mask)
		||	 gender != player->pnInfo.mGender
		||	 clan != player->pnInfo.mClan )
			{
			player->pnInfo.mRace	= race;
			player->pnInfo.mClass	= classe | (player->pnInfo.mClass & ~PlayerInfo::class_Mask);
			player->pnInfo.mGender	= gender;
			player->pnInfo.mClan	= clan;
			
			// * do something with clan-name
			
			PlayerInfo::SavePlayer( player );
			
			if ( IsPlayerBeingShown( player, -1 ) )
				gPlayersList.Update();
			
#ifdef DEBUG_VERSION
//	ShowMessage( "Saved be-info for '%s'", name );
//	ShowMessage( "/Info: '%s' (%s %s) is '%s' clan %d", name, kPlayerRaces[ race - 1],
//		kPlayerGenders[ gender ], kPlayerClass[ classe ], clan );
#endif
			}
		}
	
	return true;
}


/*
**	ParseBackEndShare()
*/
static bool
ParseBackEndShare( const char * text )
{
	// again, '-' stands for the BEPP character
	// "-be" "-sh" then
	//	 0..5 instances of "-pn" sharee-N-name "-pn" ","
	//	 "\t"
	//	 any number of "-pn" sharer-N-name "-pn" ","
	// we will already have seen the BE and the SH prefixes
	
	char name[ kMaxNameLen ];
	
	ResetSharees();
	
	// accumulate sharees (max 5)
	while ( '\t' != *text )
		{
		if ( kBEPPChar == *text )
			{
			text = ExtractPlayer( text, name );
			if ( not text )
				return true;
			
			UpdatePlayer( name, kShareeMask, kShareeMask, nullptr, true );
			
			// skip the comma
			if ( ',' == *text )
				++text;
			}
		}
	
	// skip the tab
	++text;
	
	// accumulate sharers (unlimited quantity)
	for( ;; )
		{
		// exit when we don't see another bepp
		if ( kBEPPChar != *text )
			break;
		
		text = ExtractPlayer( text, name );
		if ( not text )
			return true;
		
		UpdatePlayer( name, kSharingMask, kSharingMask, nullptr, true );
		
		// skip the comma
		if ( ',' == *text )
			++text;
		}
	
	return true;
}



/*
**	ParseBackEndWho()
*/
static bool
ParseBackEndWho( const char * text )
{
	// "-be" "-wh", followed by
	// at most twenty of:
	//	"-pn" name "-pn" "," <real-name> "," <gmlevel> "\t"
	// (the latter two will be empty unless we are a GM)
	
	char name[ kMaxNameLen ];
	char realname[ kMaxNameLen ];
	
	int gmLevel = 0;
	int numNames = 0;
	
	while ( kBEPPChar == *text )
		{
		text = ExtractPlayer( text, name );
		if ( not text )
			return true;
		
		++numNames;
		++text;		// skip first comma
		
		// extract real name if any
		const char * realNameStart = text;
		while ( ',' != *text )
			++text;
		
		if ( text == realNameStart )
			realname[0] = '\0';
		else
			BufferToStringSafe( realname, sizeof realname, realNameStart, text - realNameStart );
		
		++text;	// skip comma again
		
		// extract GMLevel
		gmLevel = atoi( text );
		
		// skip to tab separator
		while ( '\t' != *text )
			++text;
		
		// ... and past it
		++text;
		
		// remember this player
		PlayerNode * thisGuy = nullptr;
		bool changed = UpdatePlayer( name, (kBEWhoMask | gmLevel), (kBEWhoMask | kGodMask),
									 &thisGuy, true, false );
		
		// if we've already seen this player, we can stop now
		if ( not changed )
			{
//			ShowMessage( "+ already seen: %s", name );
			gNeedWho = false;
			}
		}
	
	// if we received less than 20 names in this batch, we also don't need
	// any more be-who's, since we've encountered the entire population
	if ( numNames < 20 )
		gNeedWho = false;
	
	// Redraw only after putting all new ones in the list
	gPlayersList.AdjustRange();
	if ( PlayersWindow * win = static_cast<PlayersWindow *>( gPlayersWindow ) )
		{
		win->Invalidate();
		win->AdjustCount();
		}
	
	return true;
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
**	HandleBackEnders()
**
**	consolidated handling for the responses to each of the new
**	v109 /be- commands: who, info, share
*/
static bool
HandleBackEnders( const char * text, MsgClasses * msgClass )
{
	BEPPrefixID id = GetBEP( text );
	bool suppress = false;
	
	switch ( id )
		{
		case kBEPPWhoID:
			*msgClass = kMsgWho;
			suppress = ParseBackEndWho( text + 3 );
			break;
		
		case kBEPPShareID:
			*msgClass = kMsgShare;
			suppress = ParseBackEndShare( text + 3 );
			break;
		
		case kBEPPInfoID:
			*msgClass = kMsgInfo;
			suppress = ParseBackEndInfo( text + 3 );
			break;
		
		default:
#ifdef DEBUG_VERSION
			ShowMessage( BULLET " Unknown back-end prefix" );
#endif
			break;
		}
	
	// if this message is in answer to own of our own queries, handle it
	if ( suppress )
		ProcessPlayerData( id );
	
	return suppress;
}


/*
**	ParseInfoText()
**
**	This function take an info text and breaks it up into each individual info message.
**	Each little message is then parsed by different consumers until a match is found.
**	It returns true if the messages should not be added to the text logs.
**
**	rer v79: returns message classification
**	v83: simplified, since text is only a single message at a time now
**	v90.2: take advantage of BEP prefixes from host
**	v100.1: require BEP tags
**	v141.2: stop parsing \INFOs - just bounce them back as a be-info request
**	v159: Logon and Logoff BEPs are now distinct, simplifying much
**		(ditto share/unshare, although the code doesn't take advantage of that yet)
*/
bool
ParseInfoText( const char * text, BEPPrefixID bepID, MsgClasses * msgClass )
{
	PlayersWindow *	win = static_cast<PlayersWindow *>( gPlayersWindow );
	PlayerNode * player = nullptr;
	char name[ kMaxNameLen ];
	bool suppressThisOne = false;
	const char * pName = nullptr;
	
	switch ( bepID )
		{
		case kBEPPErrID:
			// at this point the text most likely looks like:
			//	"<name> is not in the lands." -- but there are no -pn tags
			pName = strstr( text, " is not in the lands." );
			if ( pName )
				{
				BufferToStringSafe( name, sizeof name, text, pName - text );
				*msgClass = kMsgLogoff;
				UpdatePlayer( name, 0, 0, &player, false );
				if ( player )
					{
					// handle friends
					if ( player->IsFriendly() )
						*msgClass = kMsgFriendLogoff;
					else
					// optionally treat sharers & sharees as friends
					if ( gPrefsData.pdTreatSharesAsFriends )
						{
						if ( player->pnFlags & (kShareeMask | kSharingMask) )
							*msgClass = kMsgShareLogoff;
						}
					}
				
				RemovePlayer( name );
				}
			break;
		
		// post v157: log-on and log-off msgs are now distinct
		case kBEPPLogOnID:
			pName = ExtractPlayer( text, name );
			if ( not pName )
				break;
			
			UpdatePlayer( name, 0, 0, &player, true );
			*msgClass = kMsgLogon;
			
			if ( player && player->IsFriendly() )
				*msgClass = kMsgFriendLogon;
			
			// suppress message if that pref is set
			if ( gPrefsData.pdSuppressClanning )
				{
				// however, if it's a friend, consult the alternate pref
				suppressThisOne = not
					( kMsgFriendLogon == *msgClass && gPrefsData.pdNoSuppressFriends );
				}
			break;
		
		case kBEPPLogOffID:
			pName = ExtractPlayer( text, name );
			if ( not pName )
				break;
			
			// assume not-friends to start
			*msgClass = kMsgLogoff;
			
			// get player flags
			UpdatePlayer( name, 0, 0, &player, false );
			if ( player )
				{
				// handle friends...
				if ( player->IsFriendly() )
					*msgClass = kMsgFriendLogoff;
				else
				// optionally treat sharers & sharees as friends
				if ( gPrefsData.pdTreatSharesAsFriends )
					{
					if ( player->pnFlags & (kShareeMask | kSharingMask) )
						*msgClass = kMsgShareLogoff;
					}
				}
			
			// suppress message if the pref says so
			if ( gPrefsData.pdSuppressClanning )
				{
				// however, if it's a friend, consult the alternate pref
				suppressThisOne = not
					( kMsgFriendLogoff == *msgClass && gPrefsData.pdNoSuppressFriends );
				}
			
			RemovePlayer( name );
			break;
		
		
// handle sharing messages
		case kBEPPShareID:
		case kBEPPUnshareID:
			*msgClass = kMsgShare;
			
			pName = ExtractPlayer( text, name );
			if ( not pName )
				{
				// handle share messages that have no names
				// using the general handler
				TestTextForShare( text );
				win->Invalidate();
				}
			else
				{
				// look for the easy pickings first
				if ( strstr( pName, " is sharing experiences with you." ) )
					{
					UpdatePlayer( name, kSharingMask, kSharingMask, &player, true );
					*msgClass = kMsgShare;
					if ( player )
						{
						if ( player->IsFriendly() )
							*msgClass = kMsgFriendShare;
						else
						if ( player->Friendship() >= kFriendBlock )
							suppressThisOne = true;
						}
					}
				else
				if ( strstr( pName, " is no longer sharing experiences with you." ) )
					{
					UpdatePlayer( name, 0, kSharingMask, &player, true );
					*msgClass = kMsgShare;
					if ( player )
						{
						if ( player->IsFriendly() )
							*msgClass = kMsgFriendShare;
						else
						if ( player->Friendship() >= kFriendBlock )
							suppressThisOne = true;
						}
					}
				else
					{
					// punt to the general handler
					TestTextForShare( text );
					win->Invalidate();
					}
				}
				break;
		
		case kBEPPWhoID:
			*msgClass = kMsgWho;
			TestTextForWho( text );
			break;
		
		case kBEPPHasFallenID:
			{
			char monster[ kMaxNameLen ];
			char where[ 256 ];
			
			*msgClass = kMsgObit;
			if ( TestForDeathText( text, name, monster, where ) )
				{
				bool changed = UpdatePlayer( name, kDeadMask, kDeadMask, &player, true );
				
				if ( not player )
					suppressThisOne = true;
				else
					{
					// the time might not be accurate, if it's not the first death notice
					if ( changed )
						{
						// only update the time if we thought he was alive before
						player->pnDateFell = int( CFAbsoluteTimeGetCurrent() );
						}
					
					// the location (if any) is accurate
					if ( strcmp( player->pnLocFell, where ) != 0 )
						{
						StringCopySafe( player->pnLocFell, where, sizeof player->pnLocFell );
						changed = true;
						}
					if ( strcmp( player->pnKillerName, monster ) != 0 )
						{
						StringCopySafe( player->pnKillerName,
							monster, sizeof player->pnKillerName );
						changed = true;
						}
					
					// this might be redundant
					if ( changed )
						win->Invalidate();
					
					// upgrade the message class if it's a friend
					if ( player->IsFriendly() )
						*msgClass = kMsgFriendObit;
					}
				
				// suppress fallen messages
				if ( gPrefsData.pdSuppressFallen
				&&	 *msgClass != kMsgFriendObit )
					{
					suppressThisOne = true;
					
					// unless it's us that fell
					if ( gThisPlayer &&  0 == strcmp( name, gThisPlayer->descName ) )
						suppressThisOne = false;
					}
				}
			}
			break;
		
			// "et resurrexit..."
		case kBEPPNoLongerFallenID:
			{
			*msgClass = kMsgObit;
			
			pName = ExtractPlayer( text, name );
			if ( not pName )
				break;
			
			bool changed = UpdatePlayer( name, 0, kDeadMask, &player, true, false );
			
			if ( not player )
				suppressThisOne = true;
			else
				{
				player->pnLocFell[0] = '\0';
				player->pnKillerName[0] = '\0';
				
				if ( changed )
					win->Invalidate();
				
				// upgrade the message class if it's a friend
				if ( player->IsFriendly() )
					*msgClass = kMsgFriendObit;
				}
			
			// suppress fallen messages
			if ( gPrefsData.pdSuppressFallen
			&&	*msgClass != kMsgFriendObit )
				{
				suppressThisOne = true;
				}
			}
			break;
		
		case kBEPPInfoID:
			// we got a regular info line (as opposed to a be-info)
			// just echo it to text, and bounce it back to server as a /be-info request.
			// We'll defer updating the player data until that arrives; it's much easier to parse.
			*msgClass = kMsgInfo;
			if ( ExtractPlayer( text, name ) )
				SendCommand( "be-info", name, true );
			break;
		
		case kBEPPBackEndCmdID:
			// this will be a reply to one of /be-info, /be-who, /be-share
			suppressThisOne = HandleBackEnders( text, msgClass );
			break;
		
		case kBEPPDownloadID:	// does nothing; replaced by autodownload mechanism
			suppressThisOne = true;
			// DownloadURL( text );
			break;
		
		case kBEPPConfigID:
			suppressThisOne = true;
			break;
		
		case kBEPPKarmaID:
			// non-suppressible karma-related msgs
			break;
		
		case kBEPPKarmaReceivedID:
			// suppress karma messages from ignored persons
			if ( ExtractPlayer( text, name ) )
				{
				UpdatePlayer( name, 0, 0, &player, false );
				if ( player )
					{
					int friends = player->Friendship();
					if ( kFriendIgnore == friends
					||	 kFriendBlock == friends )
						{
						suppressThisOne = true;
						}
					}
				}
			break;
		
		case kBEEPBardMessageID:
			{
			int bardLevel = 0;
			if ( TestTextForBard( text, name, &bardLevel ) )
				{
				*msgClass = kMsgBardLevel;
				UpdatePlayer( name, 0, 0, &player, false, false );
				if ( player )
					{
					uchar l = 0;
					if ( bardLevel )
						l = PlayerInfo::class_Bard;
					if ( (player->pnInfo.mClass & PlayerInfo::class_Bard) != l )
						{
						player->pnInfo.mClass =
							(player->pnInfo.mClass & ~PlayerInfo::class_Bard) | l;
						PlayerInfo::SavePlayer( player );
#ifdef DEBUG_VERSION
//						ShowMessage( "Saved bard level of '%s'", name );
#endif
//						win->pwContent.Update();	// Redraw the list after we change all the info
						win->Invalidate();
						}
					}
				}
			}
			break;
		
		default:
			break;
		}
	
	return suppressThisOne;
}


/*
**	ParseThinkText()
**
**	Find the name of the thinker and set them not fallen.
*/
#if defined( MULTILINGUAL )
/*
** 	Some additions for the multilingual case:
** 	In order to be able to figure out what type of think bubble
** 	is used (think, thinkto, thinkclan or thinkgroup) the old
**	way was to parse some fixed English string inserted after the name of
**	a player. Because in the multilingual environment that string
**	(e.g. " to you") is translated and can't be used for matching,
**	we are looking for a new "BEPP"-Tag that is embedded right
**	after the name by the custom-think script of the game server.
**	If that new BEPP is not found we fallback to the old behavior and
** 	stay compatible.
** 	A new think server command might look like this:
**		Torx<BEPPCHAR>t_tt to you: hello, how are you?
*/
#endif // MULTILINGUAL

bool
ParseThinkText( DescTable * target )
{
	const char * text = target->descBubbleText;
	char name[ kMaxNameLen ];

#if defined( MULTILINGUAL )
		// Note: the following constant strings should not get translated!
		// the multilingual think system works different and these strings
		// are still needed for ClanLord and old movie compatiblity.
#endif // MULTILINGUAL
#define TO_YOU_STR			" to you"
#define TO_YOUR_CLAN_STR	" to your clan"
#define TO_YOUR_GROUP_STR	" to a group"
	const size_t toYouLen = sizeof TO_YOU_STR - 1;
	const size_t toYourClanLen = sizeof TO_YOUR_CLAN_STR - 1;
	const size_t toYourGroupLen = sizeof TO_YOUR_GROUP_STR - 1;
	
	int pos = 0;
	
#if defined( MULTILINGUAL )
	size_t tlen = strlen( text );
	// look for colon or BEPP - whatever is found first.
	while ( pos < tlen  &&  ':' != text[ pos ]  &&  kBEPPChar != text[ pos ] )
		++pos;
	
	// check if we are not the end of the string
	if ( pos == tlen )
		return false;
	
	// we found the old way, copy behavior
	if ( ':' == text[ pos ] )
		{
		if ( 0 == strncmp( text + pos - toYouLen, TO_YOU_STR, toYouLen ) )
			pos -= toYouLen;
		else
		if ( 0 == strncmp( text + pos - toYourClanLen, TO_YOUR_CLAN_STR, toYourClanLen ) )
			pos -= toYourClanLen;
		else
		if ( 0 == strncmp( text + pos - toYourGroupLen, TO_YOUR_GROUP_STR, toYourGroupLen ) )
			pos -= toYourGroupLen;
		}
	else
		{
		// if we are here, we either hit the end of the string or we found the
		// correct BEPP. In that case "pos" is already set correctly and we are done.
		}
	
#else  // ! MULTILINGUAL
	
	while ( ':' != ((const uchar *) text)[ pos ] )
		++pos;
	
	if ( 0 == strncmp( text + pos - toYouLen, TO_YOU_STR, toYouLen ) )
		pos -= toYouLen;
	else
	if ( 0 == strncmp( text + pos - toYourClanLen, TO_YOUR_CLAN_STR, toYourClanLen ) )
		pos -= toYourClanLen;
	else
	if ( 0 == strncmp( text + pos - toYourGroupLen, TO_YOUR_GROUP_STR, toYourGroupLen ) )
		pos -= toYourGroupLen;
	
#endif // MULTILINGUAL
	
	if ( pos > kMaxNameLen - 1 )
		return false;
	
	BufferToStringSafe( name, sizeof name, text, pos );
	
	// Update desc name for that bubble too
	if ( name[0] )
		{
//		ShowMessage( "Fixing think bubble for '%s'", name );
		StringCopySafe( target->descName, name, sizeof target->descName );
		}
	
	PlayerNode * p = nullptr;
	UpdatePlayer( name, 0, kDeadMask, &p, false );		// no new player entry for thoughts
	target->descPlayerRef = p;
	
	return true;
}

#pragma mark -


/*
**	SelectPlayer()
**
**	Look up the given name and make them the current selection.
**	If the name is not in the players list, add it.
*/
void
SelectPlayer( const char * name, bool toggle /* =true */ )
{
	PlayersWindow *	win = static_cast<PlayersWindow *>( gPlayersWindow );
	PlayerNode * thePlayer = nullptr;
	
	if ( name && 0 != NameComparison( name, gSelectedPlayerName ) )
		{
		UpdatePlayer( name, 0, 0, &thePlayer, true );
		
		// Just give up if thePlayer is bad...
		if ( not thePlayer )
			return;
		
		int num = thePlayer->pnNum;
		
		gPlayersList.plSelect = thePlayer;
		gPlayersList.plSelectNum = num;
		StringCopySafe( gSelectedPlayerName, thePlayer->pnName, sizeof gSelectedPlayerName );
		gPlayersList.ShowNode( num );
		win->Invalidate();
		}
	else
	if ( toggle )
		{
		int prevSelect = gPlayersList.plSelectNum;
		ClearSelection();
		if ( IsPlayerBeingShown( nullptr, prevSelect ) )
			gPlayersList.Update();
		}
	UpdateCommandsMenu( gSelectedPlayerName );
}


/*
**	GetPlayerIsFriend()
**
**	Find out if the player is a friend.
*/
int
GetPlayerIsFriend( const DescTable * desc )
{
	if ( const PlayerNode * player = LookUpPlayer( desc ) )
		return player->Friendship();
	
	return kFriendNone;
}


/*
**	GetPlayerIsFriend( char * )
**
**	Same, but works given only a name
*/
int
GetPlayerIsFriend( const char * name )
{
	PlayerNode * player = nullptr;
	UpdatePlayer( name, 0, 0, &player, 0 );
	if ( player )
		return player->Friendship();
	
	return kFriendNone;
}


/*
**	SetPlayerIsFriend()
**
**	Set the player's status as a friend.
*/
void
SetPlayerIsFriend( const DescTable * desc, int friends )
{
	if ( PlayerNode * player = LookUpPlayer( desc ) )
		{
		player->UpdateFlags( friends << kFriendShift, kFriendMask );
		SetPermFriend( player->pnName, friends );
		if ( IsPlayerBeingShown( player, -1 )
		&&	 desc != gThisPlayer )
			{
			gPlayersList.Update();
			}
		}
}


/*
**	GetNextFriendLabel()
**
**	Returns next friend flag, cycling to beginning if at end.
*/
int
GetNextFriendLabel( int prevflag )
{
	int flag = kFriendNone;
	
	if ( prevflag < 0
	||	 prevflag >= kFriendFlagCount )
		{
#ifdef DEBUG_VERSION
		ShowMessage( "Cycling invalid friend flag %d", prevflag );
#endif
		}
	// like modulo, but doesn't rely on value of kFriendLabel1
	else
	if ( kFriendNone == prevflag )
		flag = kFriendLabel1;
	else
		{
		flag = prevflag + 1;
		if ( flag > kFriendLabelLast )
			flag = kFriendNone;
		}
	
	return flag;
}


/*
**	FriendFlagNumberFromName()
**
**	Returns kFriend... flag constant from string label-name
** 	or <0 on error
**	If allLabels is true, considers all possible friend types, including blocked & ignored
**	Otherwise, only considers the 5 standard labels and "none"
*/
int
FriendFlagNumberFromName( const char * name, bool allLabels, bool showError )
{
	SafeString msg;
	
	// convert the user's command to lower-case
	char buff[ 256 ];
	StringCopySafe( buff, name, sizeof buff );
	
	StringToLower( buff );
	size_t len = strlen( buff );
	
	// search for the first match
	int i;
	int found = 0;
	for ( i = kFriendNone; i < kFriendFlagCount; ++i )
		{
		if ( allLabels
		||	 kFriendNone == i
		||	 (i >= kFriendLabel1 && i <= kFriendLabelLast) )
			{
			if ( 0 == memcmp( sFriendLabelNames[ i ], buff, len ) )
				{
				found = i;
				break;
				}
			}
		}
	
	// if have not found a match, there is none
	if ( i >= kFriendFlagCount )
		{
		if ( showError )
			{
			// "Unknown label '%s'."
			msg.Format( _(TXTCL_UNKNOWN_LABEL), buff );
			ShowInfoText( msg.Get() );
			}
		return kFriendNotFound;
		}
	
	// check if found an exact match
	if ( 0 == strcmp( sFriendLabelNames[ found ], buff ) )
		return found;
	
	// check if ambiguous
	for ( i = found + 1; i < kFriendFlagCount; ++i )
		{
		if ( allLabels
		||	 kFriendNone == i
		||	 (i >= kFriendLabel1 && i <= kFriendLabelLast) )
			{
			if ( 0 == memcmp( sFriendLabelNames[ i ], buff, len ) )
				{
				if ( showError )
					{
					// "Sorry, the label name '%s' is ambiguous."
					msg.Format( _(TXTCL_LABEL_AMBIGUOUS), buff );
					ShowInfoText( msg.Get() );
					}
				return kFriendAmbiguous;
				}
			}
		}
	
	// no match through end of list
	// not ambiguous
	return found;
}


/*
**	GiveFriendFeedback()
**
**	Gives feedback about friend-label changes made
*/
void
GiveFriendFeedback( const char * name, int oldlabel, int newlabel )
{
	SafeString msg;
	
	if ( kFriendNone == newlabel )
		{
		if ( kFriendBlock == oldlabel )
			{
			// "No longer blocking %s."
			msg.Format( _(TXTCL_NO_LONGER_BLOCKING), name );
			}
		else
		if ( kFriendIgnore == oldlabel )
			{
			// "No longer ignoring %s."
			msg.Format( _(TXTCL_NO_LONGER_IGNORING), name );
			}
		else
			{
			// "Removing label from %s."
			msg.Format( _(TXTCL_REMOVING_LABEL), name );
			}
		}
	else
	if ( kFriendBlock == newlabel )
		{
		// "Blocking %s."
		msg.Format( _(TXTCL_BLOCKING_PLAYER), name );
		}
	else
	if ( kFriendIgnore == newlabel )
		{
		// "Ignoring %s."
		msg.Format( _(TXTCL_IGNORING_PLAYER), name );
		}
	else
	if ( newlabel >= kFriendNone && newlabel <= kFriendFlagCount )
		{
		// "Labelling %s %s."
		msg.Format( _(TXTCL_LABELLING_PLAYER), name, sFriendLabelNames[ newlabel ] );
		}
	
	ShowInfoText( msg.Get() );
}


/*
**	SetFriend()
**
**	like SetPlayerIsFriend, but only works on selection.
**	plus, it's a toggle
*/
void
SetFriend( int friendType, bool feedback )
{
	PlayersWindow * win = static_cast<PlayersWindow *>( gPlayersWindow );
	if ( win )
		{
		PlayerNode * player = gPlayersList.plSelect;
		if ( not player )
			return;
		
		const int oldFriends = player->Friendship();
		int newFriends = friendType;
		
		// if player already had this classification, toggle it
		if ( oldFriends == friendType )
			newFriends = kFriendNone;
		
		player->UpdateFlags( newFriends << kFriendShift, kFriendMask );
		
		if ( feedback )
			GiveFriendFeedback( player->pnName, oldFriends, newFriends );
		
		// toggle permanency
		SetPermFriend( player->pnName, newFriends );
		UpdateCommandsMenu( player->pnName );
		if ( IsPlayerBeingShown( player, -1 ) )
			gPlayersList.Update();
		}
}


/*
**	SetFriendLabel()
**
**	like SetPlayerIsFriend, but only works on selection.
**	called from "Label" hier. menu
*/
void
SetFriendLabel( int newFriends, bool feedback )
{
	PlayersWindow * win = static_cast<PlayersWindow *>( gPlayersWindow );
	if ( win )
		{
		PlayerNode * player = gPlayersList.plSelect;
		if ( not player )
			return;
		
		const int oldFriends = player->Friendship();
		
		// do nothing if player already had this classification
		if ( oldFriends == newFriends )
			return;
		
		player->UpdateFlags( newFriends << kFriendShift, kFriendMask );
		
		if ( feedback )
			GiveFriendFeedback( player->pnName, oldFriends, newFriends );
		
		// toggle permanency
		SetPermFriend( player->pnName, newFriends );
		UpdateCommandsMenu( player->pnName );
		if ( IsPlayerBeingShown( player, -1 ) )
			gPlayersList.Update();
		}
}


/*
**	GetPlayerIsDead()
**
**	Find out if the player is fallen.
*/
bool
GetPlayerIsDead( const DescTable * desc )
{
	if ( const PlayerNode * player = LookUpPlayer( desc ) )
		return player->pnFlags & kDeadMask;
	
	return false;
}


/*
**	SetPlayerIsDead()
**
**	Set the player's status as fallen or not.
*/
void
SetPlayerIsDead( const DescTable * desc, bool dead )
{
	PlayerNode * player = LookUpPlayer( desc );
	if ( not player )
		return;
	
#ifdef DEBUG_VERSION
	if ( NameComparison( player->pnName, desc->descName ) )
		ShowMessage( BULLET " Desc doesn't match player: d=%s, p=%s",
			desc->descName, player->pnName );
#endif
	
	const bool wasDead = ((player->pnFlags & kDeadMask) != 0);
	
	if ( wasDead != dead )
		{
		player->UpdateFlags( dead ? kDeadMask : 0, kDeadMask );
		if ( dead )
			player->pnDateFell = int( CFAbsoluteTimeGetCurrent() );
		else
			{
			player->pnLocFell[0] = '\0';
			player->pnKillerName[0] = '\0';
			}
		
		// Redraw window to show them as not fallen now
		gPlayersWindow->Invalidate();
		}
}


/*
**	GetPlayerIsClan()
**
**	Find out if the player is in your clan.
*/
bool
GetPlayerIsClan( const DescTable * desc )
{
	if ( const PlayerNode * player = LookUpPlayer( desc ) )
		return player->pnFlags & kClanMask;
	
	return false;
}


/*
**	SetPlayerIsClan()
**
**	Set the player's status as a member of your clan.
*/
void
SetPlayerIsClan( const DescTable * desc, bool clan )
{
	if ( PlayerNode * player = LookUpPlayer( desc ) )
		player->UpdateFlags( clan ? kClanMask : 0, kClanMask );
}


/*
**	GetPlayerIsSharing()
**
**	Find out if the player is sharing with you.
*/
bool
GetPlayerIsSharing( const DescTable * desc )
{
	if ( PlayerNode * player = LookUpPlayer( desc ) )
		return player->pnFlags & kSharingMask;
	
	return false;
}


/*
**	SetPlayerIsSharing()
**
**	Set the player's status as sharing or not.
*/
void
SetPlayerIsSharing( const DescTable * desc, bool sharing )
{
	if ( PlayerNode * player = LookUpPlayer( desc ) )
		player->UpdateFlags( sharing ? kSharingMask : 0, kSharingMask );
}


/*
**	GetPlayerIsSharee( const DescTable * )
**
**	Find out if you are sharing with a player.
*/
bool
GetPlayerIsSharee( const DescTable * desc )
{
	if ( const PlayerNode * player = LookUpPlayer( desc ) )
		if ( player->pnFlags & kShareeMask )
			return true;
	
	return false;
}


#if 0	// not used
/*
**	GetPlayerIsSharee( const char * )
**
**	Same, but works given only a name
*/
bool
GetPlayerIsSharee( const char * name )
{
	PlayerNode * player = nullptr;
	UpdatePlayer( name, 0, 0, &player, 0 );
	if ( player && (player->pnFlags & kShareeMask) )
	 	return true;
	
	return false;
}
#endif  // 0


/*
**	SetPlayerIsSharee()
**
**	Set the player's status as being shared with or not.
*/
void
SetPlayerIsSharee( const DescTable * desc, bool sharee )
{
	if ( PlayerNode * player = LookUpPlayer( desc ) )
		player->UpdateFlags( sharee ? kShareeMask : 0, kShareeMask );
}


/*
** ResetSharees()
**
** Clear all sharee flags
*/
void
ResetSharees()
{
	for ( PlayerNode * player = gPlayersHead; player; player = player->pnNext )
		player->UpdateFlags( 0, kShareeMask );
	
	gNumSharees = 0;
}

#pragma mark -


/*
**	PlayerInfo::OpenFile()
**
**	prepare the saved player-info database file
*/
void
PlayerInfo::OpenFile()
{
	CloseFile();
	
	DTSFileSpec fs;
	fs.SetFileName( kClientPlayersFName );
	sFileError = sFile.Open( &fs, kKeyReadWritePerm | kKeyCreateFile, rClientPlayersFREF );
	
	if ( sFileError )
		return;
	
	long typeCount = 0;
	sFileError = sFile.CountTypes( &typeCount );
	
	if ( sFileError >= 0 )
		{
		int16_t version = NativeToBigEndian( int16_t( kVersion) );
		
		if ( not typeCount )
			{
			// write small header
			sFileError = sFile.Write( kHeaderID, kHeaderIndex, &version, sizeof version );
			}
		else
			{
			// read version num
			sFileError = sFile.Read( kHeaderID, kHeaderIndex, &version, sizeof version );
			version = BigToNativeEndian( version );
			if ( sFileError < 0 || version != kVersion )
				{
				// update/purge the file
				sFileError = Convert( version );
				if ( sFileError >= 0 )
					{
					version = NativeToBigEndian( int16_t(kVersion) );
					sFileError = sFile.Write( kHeaderID, kHeaderIndex, &version, sizeof version );
					}
				}
			}
		}
}


/*
**	PlayerInfo::Convert()
**	update saved player data to current format
*/
DTSError
PlayerInfo::Convert( int inVersion )
{
	long types;
	DTSError error	= sFile.CountTypes( &types );
	int total 		= 0;
	int deleted		= 0;
	int converted	= 0;
	
	if ( error != noErr || not types )
		return -1;
	
	UInt32 now;
	MyGetDateTime( &now );
	
	for ( int t = types - 1; t >= 0; --t )
		{
		DTSKeyType ttype;
		error = sFile.GetType( t, &ttype );
		
		// ignore special records
		if ( noErr == error && not IsSpecialKey( ttype ) )
			{
			long numIDs;
			error = sFile.Count( ttype, &numIDs );
			
			for ( int i = numIDs - 1; i >= 0; --i )
				{
				DTSKeyID thisID;
				error = sFile.GetID( ttype, i, &thisID );
				if ( error )
					continue;
				
				PlayerInfo info;
				bool write = false;
				error = sFile.Read( ttype, thisID, &info, sizeof info );
				
				if ( noErr == error )
					{
					++total;
					
					// do not byteswap here, since the field offsets could be wrong
					// if the record is an old one.
					
					switch ( inVersion )
						{
						case 0:
							// v0 was 2 uints shorter than v1; it lacked the
							// mGenger..mClan bitfields, as well as the mReserved field.
							char * d = reinterpret_cast<char *>( &info.mPictID );
							const char * s = reinterpret_cast<char *>( &info );
							memmove( d, s, sizeof info - (d - s) );
							info.mGender 	= 0;
							info.mRace		= 0;
							info.mClass 	= 0;
							info.mClan		= 0;
							info.mReserved	= 0;
							write = true;
							++converted;
							break;
						}
					
					if ( (now - info.mLastSeen) > kDeleteDelay )
						{
						sFile.Delete( ttype, thisID );
						++deleted;
						write = false;
						}
					
					if ( write )
						{
						// again, no byteswapping
						
						size_t len = offsetof( PlayerInfo, mName ) + 1 + info.mName[0];
						error = sFile.Write( ttype, thisID, &info, len );
						}
					}
				}
			}
		}
	if ( noErr == error && (converted || deleted) )
		{
		CompressFile();
		ShowMessage( BULLET " Updated CL_Player. %d entries, deleted %d, converted %d",
			total, deleted, converted );
		}
	
	return error;
}


#ifdef DEBUG_VERSION
/*
**	GatherPlayerFileStats()
**
**	debug helper wrapper
*/
DTSError
GatherPlayerFileStats( char * outtext )
{
	return PlayerInfo::Stats( outtext );
}


/*
**	PlayerInfo::Stats()
**
**	collect fascinating census data from our saved player-data records
**	also export those records to a text file
*/
DTSError
PlayerInfo::Stats( char * outtext )
{
	long types;
	DTSError error = sFile.CountTypes( &types );
	
	int total = 0;
	int outdated = 0;
	int gotinfo = 0;
	int bards = 0;
	
	*outtext = '\0';
	
	if ( error != noErr || 0 == types )
		return -1;
	
	UInt32 now;
	MyGetDateTime( &now );
	
	int classes[ 40 ];
	int races[ 40 ];
	FILE * stream = fopen( "CL_Players.export.txt", "w" );
	
	bzero( classes, sizeof classes );
	bzero( races, sizeof races );
	PlayerInfo info;
	
	for ( int t = types - 1; t >= 0; --t )
		{
		DTSKeyType ttype;
		error = sFile.GetType( t, &ttype );
		
		// ignore special records
		if ( noErr == error && not IsSpecialKey( ttype ) )
			{
			long numIDs;
			error = sFile.Count( ttype, &numIDs );
			
			for ( int i = numIDs - 1; i >= 0; --i )
				{
				DTSKeyID id;
				error = sFile.GetID( ttype, i, &id );
				if ( error )
					continue;
				
				error = sFile.Read( ttype, id, &info, sizeof info );
				
				if ( noErr == error )
					{
					++total;
					
					NativeToBigEndian( info );
					
					// char name[ kMaxNameLen ];
					// strncpy( name, &info.mName[1], info.mName[0] );
					// name[ info.mName[0] ] = 0;
					char classe = info.mClass & ~class_Bard;
					
					++classes[ info.mRace ? classe + 1 : 0 ];
					++races[ info.mRace ];
					
					if ( info.mRace > 0 )
						++gotinfo;
					if ( info.mClass & class_Bard )
						++bards;
					if ( (now - info.mLastSeen) > kDeleteDelay )
						++outdated;
					}
				}
			}
		}
	
	if ( stream )
		{
		// dump classes
		if ( bards )
			fprintf( stream, "Bard\t" );
		if ( classes[0] )
			fprintf( stream, "No Info\t" );
		for ( int i = 0; kPlayerClass[i]; ++i )
			{
			if ( *kPlayerClass[i] && classes[ i + 1 ] )
				fprintf( stream, "%s\t", kPlayerClass[i] );
			}
		fputc( '\n', stream );
		
		if ( bards )
			fprintf( stream, "%d\t", bards );
		if ( classes[0] )
			fprintf( stream, "%d\t", classes[0] );
		for ( int i = 0; kPlayerClass[i]; ++i )
			{
			if ( *kPlayerClass[i] && classes[ i + 1 ] )
				fprintf( stream, "%d\t", classes[ i + 1 ] );
			}
		fputc( '\n', stream );
		
		// dump races
		if ( races[ 0 ] )
			fprintf( stream, "No Info\t" );
		for ( int i = 0; kPlayerRaces[i]; ++i )
			{
			if (*kPlayerRaces[i] && races[ i + 1 ] )
				fprintf( stream, "%s\t", kPlayerRaces[i] );
			}
		fputc( '\n', stream );
		
		if ( races[ 0 ] )
			fprintf( stream, "%d\t", races[0] );
		for ( int i = 0; kPlayerRaces[i]; ++i )
			{
			if ( *kPlayerRaces[i] && races[ i + 1 ] )
				fprintf( stream, "%d\t", races[ i + 1 ] );
			}
		fputc( '\n', stream );
		
		fclose( stream );
		}
	
	if ( noErr == error )
		{
		sprintf( outtext,
			BULLET " CL_Players. %d players, %d outdated, %d with info, %d bards.",
						 total, outdated, gotinfo, bards );
		}
	
	return error;
}
#endif  // DEBUG_VERSION


/*
**	PlayerInfo::CloseFile()			[static]
*/
void
PlayerInfo::CloseFile()
{
	sFile.Close();
	sFileError = 0;
}


/*
**	PlayerInfo::CompressFile()		[static]
**
**	rebuild the CL_players file by compressing it
*/
DTSError
PlayerInfo::CompressFile()
{
	return sFile.Compress();
}


/*
**	PlayerInfo::MakeNameKey()		[static]
**
**	create a DTSKeyType for a player name. The format is:
**	First byte is total length of the actual name,
**	followed by the first three bytes of the name (padded w/0's if the name is super short)
**
**	KeyTypes with first byte 0 are for internal bookkeeping.
*/
DTSKeyType
PlayerInfo::MakeNameKey( const char * name )
{
	size_t ll = strlen( name );
	DTSKeyType res = 0;
	uchar * z = reinterpret_cast<uchar *>( &res );
	
	z[0] = static_cast<uchar>( ll );
	for ( uint i = 0;  i < 3  &&  i < ll;  ++i )
		z[ i + 1 ] = static_cast<uchar>( name[ i ] );
	
	res = NativeToBigEndian( res );
	
	return res;
}


/*
**	PlayerInfo::FindPlayerID()
**
**	attempt to retrieve saved player data for a given name
**	returns a new unused KeyID if player not found
*/
bool
PlayerInfo::FindPlayerID( const char * name, DTSKeyType& outType, DTSKeyID& outID,
					PlayerInfo& outInfo )
{
	outType	= MakeNameKey( name );
	outID = 0;
	
	size_t len = strlen( name );
	PlayerInfo info;
	while ( noErr == sFile.Exists( outType, outID ) )
		{
		DTSError err = sFile.Read( outType, outID, &info, sizeof info );
		
		if ( err != noErr )
			break;
		
		if ( len == (uint) info.mName[0]
		&&   0 == strncmp( name, &info.mName[1], len ) )
			{
			BigToNativeEndian( info );
			outInfo = info;
			return true;
			}
		++outID;
		}
	
	return false;
}


/*
**	PlayerInfo::SavePlayer()
*/
DTSError
PlayerInfo::SavePlayer( const PlayerNode * player )
{
	if ( sFileError != noErr )
		return noErr;
	
	// don't save true newbies -- they're bound to grow up and molt, eventually
	if ( player->IsNewbie() )
		return noErr;
	
	// nor other transient appearances
	if ( kBoatPlayerPict == player->pnInfo.mPictID
	||	 kNethackPlayerPict == player->pnInfo.mPictID )		// April Fools nethack "@"
		{
		return noErr;
		}
	
	// would be nice to have similar tests for ranger morphs, costume rentals, etc.
	// see comment above re other potentially "non-client-cached" pictures.
	
	// look up the player's icon-cache-file type and ID, ignoring saved info record
	PlayerInfo info;
	DTSKeyType nameID;
	DTSKeyID nameI;
	FindPlayerID( player->pnName, nameID, nameI, info );
	
	// install newest info data, dated as of this very instant
	info = player->pnInfo;
	UInt32 now;
	MyGetDateTime( &now );
	info.mLastSeen = now;
	
//	CopyCStringToPascal( player->pnName, info.mName );
	info.mName[0] = strlen( player->pnName );
	memcpy( &info.mName[1], player->pnName, info.mName[0] );
	
	// dump to file
	size_t len = offsetof( PlayerInfo, mName ) + 1 + info.mName[0];
	NativeToBigEndian( info );
	DTSError err = sFile.Write( nameID, nameI, &info, len );
	
#ifdef DEBUG_VERSION
	if ( err )
		ShowMessage( "Error %d writing entry '%s'", (int) err, player->pnName );
#endif
	
	return err;
}


/*
**	PlayerInfo::LoadPlayer()
*/
bool
PlayerInfo::LoadPlayer( PlayerNode * player )
{
	if ( sFileError != noErr )
		return false;
	
	PlayerInfo info;
	DTSKeyType nameID;
	DTSKeyID nameI;
	
	return FindPlayerID( player->pnName, nameID, nameI, player->pnInfo );
}


/*
**	NativeToBigEndian( PlayerInfo )
**	byteswap
*/
void
NativeToBigEndian( PlayerInfo& info )
{
#if DTS_BIG_ENDIAN
# pragma unused( info )
	// no-op
#else
	info.mPictID = NativeToBigEndian( info.mPictID );
	info.mLastSeen = NativeToBigEndian( info.mLastSeen );
#endif
}


/*
**	BigToNativeEndian()
**	byteswap
*/
void
BigToNativeEndian( PlayerInfo& info )
{
#if DTS_BIG_ENDIAN
# pragma unused( info )
	// no-op
#else
	info.mPictID = BigToNativeEndian( info.mPictID );
	info.mLastSeen = BigToNativeEndian( info.mLastSeen );
#endif
}

#pragma mark -


/*
**	ResolvePlayerName()
**
**	Look up the given partial name and see if it gives a unique
**	full name. Used by commands.  Returns full name in 'name'.
*/
bool
ResolvePlayerName( SafeString * name, bool showError )
{
	// Compact the name
	SafeString cName;
	cName.Set( name->Get() );
	CompactName( cName.Get(), cName.Get(), cName.Size() );
	
	// Traverse the list looking at names
	for ( const PlayerNode * thePlayer = gPlayersHead;
		  thePlayer;
		  thePlayer = thePlayer->pnNext )
		{
		if ( 0 == StartNameComparisonFixed( cName.Get(), thePlayer->pnName ) )
			{
			// Look at the next name too, to check for duplicate names
			if ( thePlayer->pnNext
			&&	 0 == StartNameComparisonFixed( cName.Get(), thePlayer->pnNext->pnName ) )
				{
				// Unless we looked at the whole name of the first one!
				// Example: "Al" and "Al azif", /select al
				SafeString compName;
				compName.Set( thePlayer->pnName );
				
				if ( strlen( compName.Get() ) != strlen( cName.Get() ) )
					{
					if ( showError )
						{
						cName.Clear();
						
						// "The partial name '%s' is not unique."
						cName.Format( _(TXTCL_PARTIAL_NAME_NOT_UNIQUE), name->Get() );
						ShowInfoText( cName.Get() );
						}
					return false;
					}
				}
			
			name->Set( thePlayer->pnName );
			return true;
			}
		}
	
	// is it an error if no name was specified?
	if ( showError
	&&	 name->Size() > 1 )
		{
		cName.Clear();
		
		// "No player named '%s' found in the player list."
		cName.Format( _(TXTCL_NO_SUCH_PLAYER_IN_LIST), name->Get() );
		ShowInfoText( cName.Get() );
		}
	
	return false;
}


/*
**	SetNamedFriend()
**
**	like SetPlayerIsFriend, but it works given a name
*/
void
SetNamedFriend( const char * name, int friendType, bool feedback )
{
	if ( gPlayersWindow )
		{
		char cName[ kMaxNameLen ];
		CompactName( name, cName, sizeof cName );
		
		// Traverse the list looking at names
		for ( PlayerNode * player = gPlayersHead; player; player = player->pnNext )
			{
			if ( 0 == NameComparisonFixed( cName, player->pnName ) )
				{
				int oldFriends = player->Friendship();
				int newFriends = friendType;
				
				player->UpdateFlags( newFriends << kFriendShift, kFriendMask );
				
				if ( feedback )
					GiveFriendFeedback( player->pnName, oldFriends, newFriends );
				
				// toggle permanency
				SetPermFriend( player->pnName, newFriends );
				UpdateCommandsMenu( player->pnName );
				if ( IsPlayerBeingShown( player, -1 ) )
					gPlayersList.Update();
				
				break;
				}
			}
		}
}


/*
**	OffsetSelectedPlayer()
**
**	Change the selection by a number
**	Returns if the selection was changed (needs to be redrawn)
*/
void
OffsetSelectedPlayer( int delta )
{
	PlayersWindow * win = static_cast<PlayersWindow *>( gPlayersWindow );
	if ( not win )
		return;
	
	int num = gPlayersList.plSelectNum;
	
	// Select the first
	if ( int(0x80000000U) == delta )
		num = 0;
	else
	// Select the last
	if ( 0x7FFFFFFF == delta )
		num = gNumPlayers - 1;
	else
		{
		// Change the selection by a number
		num += delta;
		
		// Make sure it's in range
		if ( num < 0 )
			num = 0;
		else
		if ( num >= gNumPlayers )
			num = gNumPlayers - 1;
		}
	
	PlayerNode * newPlayer = FindNumInList( num );
	if ( not newPlayer )
		return;
	
	gPlayersList.plSelectNum = num;
	gPlayersList.plSelect = newPlayer;
	StringCopySafe( gSelectedPlayerName, newPlayer->pnName, sizeof gSelectedPlayerName );
	
	// scroll selection into view
	if ( num < gPlayersList.lvStartNum )
		gPlayersList.lvStartNum = num;
	else
	if ( num >= gPlayersList.lvStartNum + gPlayersList.lvShowNum )
		gPlayersList.lvStartNum = num - gPlayersList.lvShowNum + 1;
	
	UpdateCommandsMenu( gSelectedPlayerName );
//	win->Draw();
	win->Invalidate();
	gPlayersList.AdjustRange();
}


/*
**	ListShares()
**
**	returns a string containing the "simple names" of all sharers/sharees
**	'bOutbound' == true means "those with whom I am sharing"
*/
void
ListShares( SafeString * dst, bool bOutbound )
{
	uint mask = bOutbound ? kShareeMask : kSharingMask;
	dst->Clear();
	uint nFound = 0;
	
	const PlayerNode * me = LookUpPlayer( gThisPlayer );
	
	for ( const PlayerNode * node = gPlayersHead; node; node = node->pnNext )
		{
		// don't include self among the share{e,r}s
		if ( node == me )
			continue;
		
		if ( node->pnFlags & mask )
			{
			if ( nFound )
				dst->Append( " " );
			
			// feh, reimplementing RealToSimplePlayerName() here.
			char simpleName[ kMaxNameLen ];
			char * d = simpleName;
			const char * s = node->pnName;
			
			while ( *s )
				{
				if ( isalnum( * (const uchar *) s ) )
					*d++ = * (const uchar *) s;
				++s;
				}
			*d = '\0';
			
			dst->Append( simpleName );
			
			++nFound;
			}
		}
}

