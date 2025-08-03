/*
**	Public_cl.h		ClanLord, CLServer
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

#ifndef PUBLIC_CL_H
#define	PUBLIC_CL_H

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

// turn this on to use longer, non-byte-aligned image IDs and coordinates in comm spool
#if ! defined( BITWISE_IMAGE_SPOOL )
# define BITWISE_IMAGE_SPOOL	1
#endif

// turn this on to enable extended-login structures & protocols (not yet supported)
#if ! defined( USE_EXTENDED_LOGON )
# define USE_EXTENDED_LOGON		0
#endif


/*
**	maximum size of player names, creature names, object names, and other strings
*/
enum
{
	kMaxJoinLen				= 16,	// character join-names
	kMaxAccountLen			= 16,	// account names
	kMaxPassLen				= 16,	// acct & char passwords
	kMaxNameLen				= 48,	// players and NPCs
	kMaxMonsterNameLen		= 256,	// enlarged to support Arindal's... 
	kMaxItemNameLen			= 256,	// ... multilingual name feature
	kMaxEMailLen			= 48,	// account-owner's email address
	kScriptNameLength		= 256,	// Socks script pathnames
	kMaxListTextLength		= 256,	// editor list windows
	kScriptInitLength		= 256	// init-strings passed to Socks scripts
};

// Sets of characters that are valid in player/character names, and in passwords
#define VALID_NAME_CHARS		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz -'"
#define VALID_PASSWORD_CHARS	"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"	\
								"~@#$%^&*()_+`1234567890-=[]{}\\|;':<>?,./ "
// Hence only these characters are NOT allowed in passwords:
//	0x21 exclamation ("!")
//	0x22 double-quote (")
//	0x7F DEL [which is not normally typeable]
// plus anything outside the printable ASCII range, i.e. [\x20-\x7F].
// In retrospect, it's not clear why "!" and '"' were excluded. 


// timing constants
enum
{
	// timebase (approximate) -- this isn't really a constant; the server
	// can run at whatever rate it chooses, and it never explicitly tells
	// the client how fast it's going. But this definition is, nevertheless,
	// handy to have around (as long as it's within spitting distance of
	// the truth).

	kFramesPerSecond	= 5,
	kFramesPerMinute	= kFramesPerSecond * 60,
	kFramesPerHour		= kFramesPerMinute * 60,
};


/*
**	Error messages
*/
enum
{
	kBadCharName			= -30999,	// no such char; or, proposed new-char name ill-formed
	kBadCharPass			= -30998,	// password ill-formed
	kAlreadyLoggedOn		= -30997,			// unused
	kIncompatibleVersions	= -30996,	// client newer than server
	kAccountExists			= -30995,			// unused
	kBadNewName				= -30994,			// unused
	kBadNameLen				= -30993,			// unused
	kShuttingDown			= -30992,	// shutdown in progress, no new logins allowed
	kGameNotOpen			= -30991,	// game not yet open, no logins allowed yet
	kBadCreditCard			= -30990,			// unused
	kAccountLockedOut		= -30989,	// or other (billing?) problem; call customer service
	kBadAcctName			= -30988,	// no such account
	kBadAcctPass			= -30987,	// incorrect password for acct
	kCharacterExists		= -30986,	// proposed new-char name is already in use
	kNoFreeSlot				= -30985,	// no free char slots left on this acct
	kBadAcctChar			= -30984,	// that char does not belong to your acct
	kBadBetaPass			= -30983,	// need char p/w to complete transfer to other acct
	kCharDeleted			= -30982,	// this char has been deleted
	kCharOnline				= -30981,	// cannot modify char whilst online
	kBetaEnded				= -30980,	// trying to log into a detached char
	kCharNoDelete			= -30979,	// can't delete demo chars, or other locked ones
	kAccountExpired			= -30978,	// you must renew your subscription
	kBadQualityPass			= -30977,	// password doesn't meet our security standards
	kDemoCharacter			= -30976,	// can't modify demo characters
	kBadIdentifiers			= -30975,	// haven't yet received user idents
	kDemoNoChangePwd		= -30974,	// can't change a demo-char's p/w
	kDownloadNewVersionTest	= -30973,			// (not sent by server)
	kDownloadNewVersionLive	= -30972,	// client or data files too old; needs update
	kMachineLockedOut		= -30971,	// you've been blacklisted
	kImageChecksumError		= -30970,	// CL_Images corrupt or altered
	kBadAcctCmd				= -30969,	// can't parse acct-port cmd, or internal error
	kCharAlreadyDetached	= -30968,	// from "/detachChar"
};


/*
**	Message packets transmitted 'twixt client & server.
**	Keep the size reasonable to avoid IP fragmentation.
**	(Clearly, packet size limits can vary with path MTU, so for now
**	we start with the size of an ethernet frame (1500 bytes) as our top limit,
**	then allow 100 bytes or so of extra overhead & slop space, yielding 1400.)
*/
const int kMaxMsgSize		= 1400 - 4;				// was 1020
const int kMaxDataSize		= kMaxMsgSize - 16;		// 16 is offsetof(LogOn,logData)
#if USE_EXTENDED_LOGON
const int kMaxDataSizeExt	= kMaxMsgSize - 24;		// 24 is offsetof(LogOnExt,logData)
#endif
const int kChallengeLen		= 16;					// encrypted bytes for pwd validation


//	tags for the messages
//	make sure the assigned _values_ don't change, even if you alter the ordering
enum MsgTag
{
	kMsgNone				= 0,
//	kMsgLogOnObsolete		= 1,	// unused
	
	kMsgDrawState			= 2,
	kMsgPlayerInput			= 3,
	
	// tags 4-6 were part of an earlier protocol
	// tags 7-12 used the LogOnOBSOLETE record (and are now disused)
	
	// these all use the LogOn record
	kMsgLogOn				= 13,
	kMsgCharList			= 14,
	kMsgNewChar				= 15,
	kMsgDeleteChar			= 16,
	kMsgNewPassword			= 17,
	kMsgChallenge			= 18,
	kMsgIdentifiers			= 19,
	
#if USE_EXTENDED_LOGON
	// these all use the LogOnExtended record (eventually)
	kMsgLogOnExt			= 20,
	kMsgCharListExt			= 21,
	kMsgNewCharExt			= 22,
	kMsgDeleteCharExt		= 23,
	kMsgNewPasswordExt		= 24,
	kMsgChallengeExt		= 25,
	kMsgIdentifiersExt		= 26
#endif  // USE_EXTENDED_LOGON
};


#pragma pack( push, 2 )

/*
**	LogOn record
**
**	log-on message sent from the client to the server
**	and from the server to the client.
**	Also handles password changes and other CharMgr tasks.
*/
struct LogOn
{
	uint16_t	logMsgTag;					// always kMsgLogOn, kMsgCharList, kMsgNewChar
	int16_t		logResult;					// result of logging on
	uint32_t	logClientVersion;			// client/server version
	uint32_t	logImagesVersion;			// images data file version
	uint32_t	logSoundsVersion;			// sounds data file version
	char		logData[ kMaxDataSize ];	// character name/password, error text, etc.
};


// Some account status codes that can be exposed to client
enum
{
	kAcctStatusNormal		= 0,
	kAcctStatusProblem		= 1,			// billing, blocked, email Joe...
	kAcctStatusDemoAcct		= 2				// try before buy (Arindal)
};


#if USE_EXTENDED_LOGON
/*
**	ClientID
**	identifies a specific "flavor" of client.
**	Each "flavor" has a current build (or version) number.
**	The combination of { ClientID, buildNumber } should uniquely name each client.
**	Whenever a new client needs to be pushed out to the users via autoupdate,
**	the minimum-required version for that clientID is updated in the server's tables.
**	Thus the Java client, say, can be forcibly updated, independent of
**	the Mac & Windows ones.
**
**	FIXME: I'm not sure if this is the best breakdown. It might need to be
**	more specific, for example to separate MacOS into 68K, PPC-CFM, PPC-MachO,
**	OpenGL, Mac-on-Intel, etc.
**	We might also want to encode a "vendor" (DT, Arindal, 3rd parties...) somehow.
*/
enum ClientID
{
	kClientIDMacOS,				// DeltaTao's Mac clients (68K, ppc, osx, ogl...)
	kClientIDWindows,			// Arindal's Windows client
	kClientIDJava,				// Arindal's Java client
	kClientIDTest				// internal, unpublished testing clients
};


/*
**	LogOnExtended class
**
**	log-on message sent from the client to the server
**	and from the server to the client.
**	Also handles password changes and other CharMgr tasks.
**	Contains new [v400+] fields for better client auto-updating
*/
struct LogOnExtended
{
	uint16_t	logMsgTag;					// always kMsgLogOnExt, kMsgCharListExt,
											//	kMsgNewCharExt
	int16_t		logResult;					// result of logging on
	uint32_t	logProtocolVersion;			// version of network protocol
	uint32_t	logClientIdentifier;		// a ClientID; see above
	uint32_t	logClientRevision;			// build number for this ClientID
	uint32_t	logImagesVersion;			// images data file version
	uint32_t	logSoundsVersion;			// sounds data file version
	char		logData[ kMaxDataSizeExt ]; // character name/password, error text, etc.
};
#endif  // USE_EXTENDED_LOGON


/*
**	PlayerInput class
**
**	player input message, sent from the client to the server
**	describes what the user wants to do this turn
*/
const int	kPlayerInputStrLen	= 512;

// Bits in piFlags
const int	kPIMDownField		= 0x0001;		// mouse down; player wants to move

struct PlayerInput
{
	uint16_t	piMsgTag;					// always kMsgPlayerInput
	int16_t		piMouseH;					// mouse location
	int16_t		piMouseV;					// mouse location
	uint16_t	piFlags;					// flags
	int32_t		piAckFrame;					// latest frame received
	int32_t		piResendFrame;				// earliest missed frame
	uint32_t	piCommandNum;				// number of this command
	char		piKeyString[ kPlayerInputStrLen ];	// cstring
};


/*
**	Message union of all messages exchanged between the client and the server
**	(at the network level, packets consist of a two-byte length,
**	followed by one of these)
*/
union Message
{
	uint16_t			msgTag;
								// leave room for packet length
	uchar				msgData[ kMaxMsgSize - sizeof(int16_t) ];
	LogOn				msgLogOn;
#if USE_EXTENDED_LOGON
	LogOnExtended		msgLogOnExt;
#endif
	PlayerInput			msgPlayerInput;
};

#pragma pack( pop )


/*
**	Bubble speech types
**
**	the different types of bubble speech
*/
enum
{
	kBubbleNormal			= 0,
	kBubbleWhisper			= 1,
	kBubbleYell				= 2,
	kBubbleThought			= 3,
	kBubbleRealAction		= 4,
	kBubbleMonster			= 5,
	kBubblePlayerAction		= 6,
	kBubblePonder			= 7,	// uses the same graphic as kBubbleThought
	kBubbleNarrate			= 8,
	// limited to <64
	
	kNumBubbleTypes			= 6,	// width, in columns, of bubble image
									// (some types share the same image)
	
	kBubbleHalfling			= 0,
	kBubbleSylvan			= 1,
	kBubblePeople			= 2,
	kBubbleThoom			= 3,
	kBubbleDwarf			= 4,
	kBubbleGhorakZo			= 5,
	kBubbleAncient			= 6,
	kBubbleMagic			= 7,
	kBubbleCommon			= 8,
	kBubbleThieves			= 9,
	kBubbleMystic			= 10,
	
	//
	// NOTE: if you add/modify languages, you must also update the kLangXXX values
	// in editor/public/Public_cle.h, plus scripts/constants/system.h
	//
	
#ifndef ARINDAL
	kBubbleNumLanguages		= 11,
	// limited to <64
#else
	kBubbleLangMonster	 	= 11,
	kBubbleLangUnknown		= 12,
	kBubbleOrga				= 13,
	kBubbleSirrush			= 14,
	kBubbleAzcatl			= 15,
	kBubbleLepori			= 16,
	kBubbleNumLanguages		= 17,
#endif  // ARINDAL
	
	// low-order 7 bits of bubble-type define its actual type (normal..narrate);
	// above that are flags
	kBubbleTypeMask			= 0x3F,
	kBubbleNotCommon		= 0x40,		// spoken in a language other than Common
										// (now called Standard); the type-code byte
										// will be followed by a lang-code byte.
	kBubbleFar				= 0x80,		// speaker of bubble is not visible in game window;
										// type-code byte will be followed by H & V offset
										// coords (16 bits each) for where to put the bubble
	
	// low 7 bits of language byte define non-Standard language, from list above
	kBubbleLanguageMask		= 0x3F,
	
	// next 2 bits encode 4 possibilities.
	// (Ever watched a foreign film where you don't speak the language, and some
	// character delivers what's clearly a whole paragraph of speech, yet the subtitle
	// just reads "See you later", or something equally terse? Infuriating!
	// So if your character doesn't understand a CL language, but he/she overhears
	// someone speaking it, you ought to be able to at least know if it's a grunt
	// or a dissertation or something in between.)
	kBubbleCodeMask			= 0xC0,
	kBubbleCodeKnown		= 0x00,		// you understand the language being spoken
	kBubbleUnknownShort		= 0x40,		// don't understand, but it was just a word or 3
	kBubbleUnknownMedium	= 0x80,		// ditto, but it was a moderately long utterance
	kBubbleUnknownLong		= 0xC0,		// ditto, and it was quite a mouthful!
	
	// a yell coming from somewhere "offscreen"
	kBubbleFarYell			= kBubbleYell | kBubbleFar,	// uses the same graphic as kBubbleYell
	
	// server will perform automatic, listener-dependent grammar substitutions.
	// Not relevant for client.
	kBubbleEscapes			= 0x80000000
};

	
// the standard size of the player icons in pixels
// (approximately, but it differs by race)
const int kPlayerSize		= 32;

// mobile icons have from 33 to 48 "poses": 4 in each of the 8 compass directions,
// one for being fallen, and up to 15 additional "roleplay" poses.

enum
{
	kPoseStand	= 0,		// standing still
	kPoseWalk1,				// walking, frame 1
	kPoseWalk2,				// walking, frame 2
	kPosePunch,				// attacking
	kPoseMax				// max number of mobile action
};

enum
{
	kFacingEast		= 0,
	kFacingSE,
	kFacingSouth,
	kFacingSW,
	kFacingWest,
	kFacingNW,
	kFacingNorth,
	kFacingNE
};

// derive a pose number, given action and direction
#define kGetMobilePose( action, facing )		( (facing) * kPoseMax + (action) )

// additional poses (not all mobile images contain all these poses, and
// certain others don't match the names. But all standard player icons do.)
// ... except that whoever originally made up these names got "akimbo" wrong:
//	that word is defined as "with hands on hips, and elbows turned out".
//	The poses whose graphics match that definition are typically labeled
//	kPoseAngry (#42) whereas the poses named "akimbo" (38) would probably be
//	better described as "arms crossed". But it's too late to change things now
//	(although we could deprecate all uses of the word "akimbo" in favor of a
//	new preferred alternative such as "arms folded").
enum
{
	kPoseDead		= 32,
	kPoseCelebrate,
	kPoseSit,
	kPoseSeated,
	kPoseLeanLeft,
	kPoseLeanRight,
	kPoseAkimbo,
	kPoseBless,
	kPoseKneel,
	kPoseLie,
	kPoseAngry,
	kPoseSalute,
	kPoseBow,
	kPoseThoughtful,
	kPoseCry,
	kPoseSurprised
};


/*
**	illumination flags in the draw-state header
*/
enum
{
	kLightAdjust25Pct  = 1 << 0,
	kLightAdjust50Pct  = 1 << 1,
	kLightAreaIsDarker = 1 << 2,
	kLightNoNightMods  = 1 << 3,
	kLightNoShadows    = 1 << 4,
	kLightForce100Pct  = 1 << 5		// override client max-night pref,
		// for "pitch black" areas. When this bit is on, clients should
		// pretend that the max-night pref is set to 100%
};


/*
**	back end protocol prefixes, in info-text stream messages
*/
#define kBEPPCharacter			"\302"		// BEPP lead-in character (MacRoman not-sign)
#define kBEPPBard				kBEPPCharacter "ba"
#define kBEPPBackEndCmd			kBEPPCharacter "be"
#define kBEPPClanName			kBEPPCharacter "cn"
#define kBEPPConfig				kBEPPCharacter "cf"
#define kBEPPDemo				kBEPPCharacter "de"
#define kBEPPDepart				kBEPPCharacter "dp"
#define kBEPPDownload			kBEPPCharacter "dl"
#define kBEPPErr				kBEPPCharacter "er"
#define kBEPPGameMaster			kBEPPCharacter "gm"
#define kBEPPHasFallen			kBEPPCharacter "hf"
#define kBEPPNoLongerFallen		kBEPPCharacter "nf"
#define kBEPPHelp				kBEPPCharacter "hp"
#define kBEPPInfo				kBEPPCharacter "in"
#define kBEPPInventory			kBEPPCharacter "iv"
#define kBEPPKarma				kBEPPCharacter "ka"
#define kBEPPKarmaReceived		kBEPPCharacter "kr"
#define kBEPPLogOff				kBEPPCharacter "lf"
#define kBEPPLogOn				kBEPPCharacter "lg"
#define kBEPPLocation			kBEPPCharacter "lo"
#define kBEPPMultilingual		kBEPPCharacter "ml"
#define kBEPPMonsterName		kBEPPCharacter "mn"
#define kBEPPMusic				kBEPPCharacter "mu"
#define kBEPPNews				kBEPPCharacter "nw"
#define kBEPPPlayerName			kBEPPCharacter "pn"
#define kBEPPShare				kBEPPCharacter "sh"
#define kBEPPUnshare			kBEPPCharacter "su"
#define kBEPPThink				kBEPPCharacter "th"
#define kBEPPWho				kBEPPCharacter "wh"
#define kBEPPYouKilled			kBEPPCharacter "yk"


// inventory commands in the state data
enum
{
	kInvCmdNone,		// no extra data
	kInvCmdFull,		// all of the inventory data
	kInvCmdAdd,			// add one item, with its name, to the inventory unequipped
	kInvCmdAddEquip,	// add one item, with its name, to the inventory and equip it
	kInvCmdDelete,		// delete one item from the inventory
	kInvCmdEquip,		// equip   one item that's in the inventory
	kInvCmdUnequip,		// unequip one item that's in the inventory
	kInvCmdMultiple,	// spec'd but not used by the v120 server; used by v240 server
	kInvCmdName,		// set the name of one item
	
	kInvCmdIndex 	= 0x80	// to be or'd with one of the above partial commands
};


#pragma pack( push, 2 )

/*
**	SimpleRect has the same memory format and layout as a DTSRect
**	(or a MacOS/QuickDraw Rect, for that matter), but has no methods.
**	It's used when the data will be serialized to disk or network,
**	to avoid C++ language issues (v-tables, non-POD types, etc.)
*/
struct SimpleRect
{
	int16_t			rectTop;
	int16_t			rectLeft;
	int16_t			rectBottom;
	int16_t			rectRight;
};

#if defined( __MACTYPES__ )
	// convenience
inline	     Rect *	DTS2Mac(       SimpleRect * r )
	{ return reinterpret_cast<Rect *>( r ); }
inline const Rect *	DTS2Mac( const SimpleRect * r )
	{ return reinterpret_cast<const Rect *>( r ); }
#endif  // __MACTYPES__


/*
**	Layout class: describes client window geometry
*/
struct Layout
{
	int32_t			layoWinHeight;
	int32_t			layoWinWidth;
	int32_t			layoPictID;
	DTSRect			layoFieldBox;	// these should become SimpleRect,
	DTSRect			layoInputBox;	// but the code gets messy
	DTSRect			layoHPBarBox;
	DTSRect			layoSPBarBox;
	DTSRect			layoAPBarBox;
	DTSRect			layoTextBox;
	DTSRect			layoLeftObjectBox;
	DTSRect			layoRightObjectBox;
};


// out of a possible 30 custom colors,
// we currently use only 21
const int	kNumPlyColors	= 30;	// should have been 32... but, oh well, too late now.
const int	kNumCustColors	= 20;

// Color Table support

//
// Each 32-byte entry in the gStandardClut table is, in reality,
// one of these RGBColor8 objects.
//
struct RGBColor8
{
#if DTS_BIG_ENDIAN
	uint8_t unused;		// placeholder for alpha
	uint8_t	red;
	uint8_t	green;
	uint8_t blue;
#elif DTS_LITTLE_ENDIAN
	uint8_t blue;
	uint8_t	green;
	uint8_t	red;
	uint8_t unused;		// placeholder for alpha
#else
# error "Unknown endianness!"
#endif
};


/*
**	Lighting data
**	for images that emit light
*/
struct LightingData
{
	union
		{
		uint8_t	ilColor[4];	// RGBA, ordered such that ilColor[0] is red.
					// (this is _not_ an RGBColor8!)
					// only RGB used for lightcasters (A will be ignored; default to 255)
					// only A used for darkcasters (for which RGB will always be 0,0,0)
		uint32_t ilCValue;	// as 32-bit bigendian
		};
	uint16_t	ilRadius;	// in pixels; normally zero, which means use width of image
	int16_t		ilPlane;	// similar to PictDef::pdPlane
};


/*
**	Picture Definition class
**
**	define the data in a PDef record
*/
const int kPictDefVersion		= 0x100; // v1.0 (24/8-bit numbering, akin to main number)
const int kPictDefAnimTableSize	= 16;

//
// Values of the pdFlags field:
//
enum
{
	// hi-order 16 bits not yet claimed
	
		// this image is not opaque (white pixels are transparent)
	kPictDefFlagTransparent			= 0x8000,
	
		// prefer better-quality blitters (when translucent)
	kPictDefFlagEffect				= 0x4000,
	
		// has custom color-index lookup map in first pixel row
	kPictDefCustomColors			= 0x2000,
	
		// shadows are drawn differently (or not at all) at "night"
	kPictDefIsShadow				= 0x1000,
	
		// this mobile "stands upright"; use a different pose for shadows
	kPictDefFlagUprightShadow		= 0x0800,
	
		// ignore any checksum errors for this image
	kPictDefFlagNoChecksum			= 0x0400,
	
		// this image emits light (torches, sparkles, etc.)
	kPictDefFlagEmitsLight			= 0x0200,
	
		// emits light sporadically: for mobiles whose attack
		// poses (only) are lit, for example #65 wizards.
	kPictDefFlagOnlyAttackPosesLit	= 0x0100,
	
		// allow random jitter in light position/intensity
	kPictDefFlagLightFlicker		= 0x0080,
	
		// this image emits darkness rather than light
	kPictDefFlagLightDarkcaster		= 0x0040,
	
	// bits 3-5 not yet assigned
	
		// show anim frames in random order (e.g. lightning)
	kPictDefFlagRandomAnimation		= 0x0004,
	
		// low-order 2 bits determine blending/translucency (see below)
	kPictDefBlendMask				= 0x0003,
	
	kPictDefFlagsDefault			= kPictDefFlagTransparent	// the "usual value"
};

enum
{
	// blending mode, from 2 low-order bits of PictDef pdFlags
	kPictDefNoBlend,				// fully opaque
	kPictDef25Blend,
	kPictDef50Blend,
	kPictDef75Blend					// least opaque
};

struct PictDef
{
	uint32_t    pdVersion; 		// see kPictDefVersion
	int32_t	    pdBitsID;		// ID of associated kTypePictureBits record
	int32_t	    pdColorsID;		// ID of associated kTypePictureColors record
	uint32_t	pdChecksum;		// cksum of all image data
	uint32_t    pdFlags;    	// enumerated above
	uint32_t    pdUnusedFlags;  // future expansion
	uint32_t    pdUnusedFlags2;	// future expansion
	int32_t	    pdLightingID;	// ID of associated kTypeLightingData record (0 if none)
	int16_t     pdPlane;		// -15 .. +15, from deepest to shallowest
	int16_t     pdNumFrames;	// number of distinct animation "cells"
	int16_t     pdNumAnims;		// number of used entries in frame-table
	int16_t     pdAnimFrameTable[ kPictDefAnimTableSize ];	// must be last
};


#if ! CL_SERVER
class CLImage
{
public:
	DTSImage		cliImage;			// essentially a pixmap
	DTSKeyID		cliPictDefID;		// ID of my PictDef
	PictDef			cliPictDef;			// contents of my PictDef
	LightingData	cliLightInfo;		// lighting data
	
					// in Utilities_cl.cp
	DTSError	CalculateChecksum( DTSKeyFile * file, uint32_t * sum );
	DTSError	CalculateChecksum( DTSKeyFile * file, const void * bits,
									const void * colors, const void * light, uint32_t * sum );
	bool		HasLightingData() const
		{
		// return 'true' if there's any significant lighting data
		return cliLightInfo.ilCValue != 0
			|| cliLightInfo.ilRadius != 0
			|| cliLightInfo.ilPlane  != 0;
		}
};
#endif  // ! CL_SERVER

#pragma pack( pop )


//	Images sent to client are packed:
//	the high-order bits contain the image ID (unsigned)
//	followed by the relative H and V coords (signed)
//	In the old word-aligned format, each image spec occupies 32 bits (12 + 10 + 10)
//	In the new (BITWISE_IMAGE_SPOOL) format, they occupy 36 bits (14 + 11 + 11).
//	This supports image IDs up to 16383, and H/V coords from -1024 to +1023.
//	(It's important that the range of H/V coords be at least as large
//	as the greater of the Layout::layoFieldBox's width or height.
//	Otherwise you get the "Rocks of Mystery" bug.)
//
#if BITWISE_IMAGE_SPOOL
const int	kPictFrameCoordNumBits	= 11;
#else
const int	kPictFrameCoordNumBits	= 10;
#endif  // BITWISE_IMAGE_SPOOL

#if BITWISE_IMAGE_SPOOL
const int	kPictFrameIDNumBits		= 14;
#else
const int	kPictFrameIDNumBits		= 32 - 2 * kPictFrameCoordNumBits;			// 12
#endif  // BITWISE_IMAGE_SPOOL

const int kPictFrameCoordLimit	= (1 << (kPictFrameCoordNumBits - 1)) - 1;	// = 511/1023 (+/-)
const int kPictFrameCoordMask	= (1 << kPictFrameCoordNumBits) - 1;
const int kPictFrameIDLimit		= (1 << kPictFrameIDNumBits) - 1;			// = 4095/16383


/*
**	DataSpool class
**
**	When using unaligned bit-manipulation functions on the spool,
**	dsMark holds the number of the byte in which the next bit will
**	be written or read, and dsBitMark holds the number of bits
**	within that byte that have already been written or read.
**	Thus if dsMark is 1 and dsBitMark is > 0, some bits in byte 2
**	have already been read or written, but not all of them.
**	The spool is byte-aligned when dsBitMark == 0.
*/

// codes for DataSpool PutNumber() and GetNumber() operations
enum DataSpoolNumberType
{
	kSpoolSignedByte,
	kSpoolUnsignedByte,
	kSpoolSignedShort,
	kSpoolUnsignedShort,
	kSpoolSignedLong,
	kSpoolUnsignedLong
};

class DataSpool
{
private:
	DTSError	dsResult;	// enumerated below
	size_t		dsMax;		// allocated size of dsData
	size_t		dsLimit;	// used size of dsData
	char *		dsData;		// data buffer
	size_t		dsMark;		// see comments above
	int			dsBitMark;	// see comments above
	
public:
	// constructor/destructor
					DataSpool();
					~DataSpool();
	
private:
	// declared but not defined
					DataSpool( const DataSpool& );
					DataSpool operator=( const DataSpool& );
	
public:
	// interface
	DTSError		Init( size_t maxSize );
		// byte-aligned operations
	void			PutData( const void * buffer, size_t size );
	void			PutString( const char * str );
	void			PutNumber( int number, DataSpoolNumberType type );
	void			GetData( void * buffer, size_t size );
	void			GetString( char * str, size_t size );
	int				GetNumber( DataSpoolNumberType type );
		// not byte-aligned (call ByteAlignXXX() before mixing with byte-aligned functions)
	void			PutBits( int value, uint numbits );
	int				GetBits( uint numbits );
	void			ByteAlignWrite();
	void			ByteAlignRead();
	
	// inline interface
	size_t			GetMark() const				{ return dsMark; }
	void			SetMark( size_t mark )		{ dsMark = mark; }
	size_t			GetLimit() const			{ return dsLimit; }
	void			SetLimit( size_t limit )	{ dsLimit = limit; }
	void *			GetData() const				{ return static_cast<void *>( dsData ); }
	DTSError		GetResult() const			{ return dsResult; }
	void			ClearResult()				{ dsResult = noErr; }
};

// DataSpool dsResult codes
enum
{
	kSpoolErrNoErr		=  0,			// all is well
	kSpoolErrFull		= -1,			// no room to Put more data
	kSpoolErrEmpty		= -2,			// no more data to Get
	kSpoolErrNotAligned	= -3			// you forgot to call ByteAlignXXX()
};


#if defined( MULTILINGUAL )
/*
**	Linguistic genders
*/
enum
{
	kLangGenderNeuter,
	kLangGenderMasculine,
	kLangGenderFeminine
};
#endif  // MULTILINGUAL

/*
**	Real language IDs
*/
#if defined( ARINDAL ) || defined( MULTILINGUAL )
const int	kRealLangDefault		= 1;	// default language spoken by NPCs
const int	kRealLangFirstAllowed	= 1;	// smallest ID /be-lang will accept
#else
const int	kRealLangDefault		= 0;
const int	kRealLangFirstAllowed	= 0;
#endif  // ARINDAL || MULTILINGUAL


/*
**	Player Inventory Items
*/
enum
{
		// "pseudo" slots
	kItemSlotNotInventory,		// this is not an inventory item
	kItemSlotNotWearable,		// item cannot be equipped
	
		// "real" slots
	kItemSlotForehead,			// sunstone
	kItemSlotNeck,				// necklaces - they're small
	kItemSlotShoulder,			// shoulders
	kItemSlotArms,				// set  of arms  - bracers cover both arms
	kItemSlotGloves,			// pair of hands - gloves  cover both hands
	kItemSlotFinger,			// ring fingers
	kItemSlotCoat,				// coat
	kItemSlotCloak,				// cloak
	kItemSlotTorso,				// shirt, breastplate, dress
	kItemSlotWaist,				// belt, cumberbund
	kItemSlotLegs,				// set of legs  - pants cover both legs
	kItemSlotFeet,				// pair of feet - boots cover both feet
	kItemSlotRightHand,			// right hand
	kItemSlotLeftHand,			// left hand
	kItemSlotBothHands,			// - for two handed items - not yet supported
	kItemSlotHead,				// hat, helmet, tiara, halo
	
		// synonyms
	kItemSlotFirstReal = kItemSlotForehead,
	kItemSlotLastReal  = kItemSlotHead
};


/*
**	subset of ItemData, without the parts we don't want
**	snoopers to see. These live in the images file, as kTypeClientItem
*/
struct ClientItem
{
	uint32_t		itemFlags;				// only the kItemTypeHasData flag
	int32_t			itemSlot;				// one of kItemSlotWhatever, above
	int32_t			itemRightHandPictID;	// picture in right hand panel
	int32_t			itemLeftHandPictID;		// picture in left  hand panel
	int32_t			itemWornPictID;			// picture in inventory list
	char			itemName[ kMaxItemNameLen ];	// public name
};


/*
**	Mobile name-label color coding
**
**	colors = ( background << 4 ) + ( text << 2 ) + ( style )
*/
enum
{
	kColorCodeBackShift = 4,
	kColorCodeTextShift = 2,
	
	// four bits (8 values) for back color
	kColorCodeBackWhite = 0,	// healthy
	kColorCodeBackGreen,		// good
	kColorCodeBackYellow,		// hurting
	kColorCodeBackRed,			// near death
	kColorCodeBackBlack,		// dead
	kColorCodeBackBlue,			// npc
	kColorCodeBackGrey,			// ghosting GM
	kColorCodeBackCyan,			// GM that looks like a monster
	kColorCodeBackOrange,		// disconnected player or afk player
	kColorCodeBackCount,
	
	// two bits for text color
	kColorCodeTextBlack = 0,	// normal text
	kColorCodeTextWhite,		// dead
	kColorCodeTextRed,			// bad karma
	kColorCodeTextBlue,			// excellent karma
	kColorCodeTextCount,
	
	// two bits for text style
	kColorCodeStylePlain = 0,	// normal text
	kColorCodeStyleBold,		// sharing with you
	kColorCodeStyleItalic		// in your clan
};


/*
**	descriptor types
*/
enum
{
	kDescUnknown,
	kDescPlayer,
	kDescMonster,
	kDescNPC,
	kDescThoughtBubble			// client synthesizes this type for thoughts
};


/*
**	class SafeString
**
**	this string grows to be as large as it needs to be
**	it also handles simple printf-style formatting: %d %s etc.
*/
class SafeString
{
	// ideally all the data members would be private, but alas there's (just)
	// one pesky place in the server's Socks runtime engine where the 'ssPtr'
	// field needs to be visible
public:
	char *	ssPtr;		// pointer to the string
private:
	size_t	ssSize;		// length including terminating null
	size_t	ssMax;		// maximum size currently allocated
	
public:
	// constructor/destructor
				SafeString();
				~SafeString();
	
	// private interface
	DTSError	ExpandTo( size_t size );
	
	// public interface
	void		Clear();
	char *		Get() const					{ return ssPtr; }
	size_t		Size() const				{ return ssSize; }
	void		Set( const char * str )		{ Clear(); Append( str ); }
	void		Set( int value )			{ Clear(); Append( value ); }
	void		Append( const char * str );
	void		Append( int value );
				// note, these two -append- to the string; they don't clear it first
	void		Format( const char * format, ... ) PRINTF_LIKE( 2, 3 );
	void		VFormat( const char * format, va_list params ) PRINTF_LIKE( 2, 0 );
	DTSError	Read( DTSKeyFile * file, DTSKeyType type, DTSKeyID keyID );
	
private:
	// declared but not defined; no copying allowed
				SafeString( const SafeString& );
	SafeString&	operator=( const SafeString& );
	
	static char	gSafeStringZero;
};


/*
**	Shared routines
**	... but some are not universally present. 
**	Notably, the server needs nothing from ImageComp_cl.cp
*/

#if ! CL_SERVER
// ImageComp_cl.cp
DTSError	LoadImage( CLImage * image, DTSKeyFile * file, int numColors = 0,
					   const uchar * customColors = nullptr );

DTSColor	GetCLColor( uint8_t idx );
const RGBColor8 *	GetCLColorTable();
CGImageRef	CGImageCreateFromDTSImage( const DTSImage * img, uint32_t pdFlags );
DTSError	LoadImage( const PictDef& pdef, CGImageRef * oImg, DTSKeyFile * file,
				int numColors = 0, const uchar * colors = nullptr );
DTSError	LoadImageX( DTSKeyID picID, CGImageRef * oImg, DTSKeyFile * file,
				bool transparent = false, int numColors = 0,
				const uchar * colors = nullptr );

# if defined( DEBUG_VERSION ) || defined( DEBUG_VERSION_TAGS )
DTSError	TaggedReadAlloc( DTSKeyFile * file, DTSKeyType type, DTSKeyID keyID,
							 void *& buffer, const char * tag );
# else
static inline DTSError
TaggedReadAlloc( DTSKeyFile * file, DTSKeyType type, DTSKeyID keyID,
				 void *& buffer, const char * /*tag*/ )
{
	return file->ReadAlloc( type, keyID, buffer );
}
# endif  // DEBUG_VERSION

// these next four are only in the Editor, not in the client or server
DTSError	StoreImage( CLImage * image, DTSKeyFile * file );
DTSError	RemoveImage( DTSKeyFile * file, DTSKeyID pictID );
DTSError	GetImageSize( DTSKeyFile * file, DTSKeyID pictID, int * width, int * height );
DTSError	GetImageNumColors( DTSKeyFile * file, DTSKeyID bitsid, int * numColors );


# if DTS_BIG_ENDIAN
inline void	NativeToBigEndian( PictDef * ) {}
inline void	BigToNativeEndian( PictDef * ) {}
inline void	NativeToBigEndian( LightingData * ) {}
inline void	BigToNativeEndian( LightingData * ) {}
# else
void		NativeToBigEndian( PictDef * );
void		BigToNativeEndian( PictDef * );
void		NativeToBigEndian( LightingData * );
void		BigToNativeEndian( LightingData * );
# endif  // DTS_BIG_ENDIAN

#endif  // ! CL_SERVER


// MessageWin_cl.cp
void		CreateMsgWin( int numLines, int kind, const DTSRect * box );
void		CloseMsgWin();
void		GetMsgWinPos( DTSPoint * pos, DTSPoint * size = nullptr );
void		ShowMessage( const char * format, ... ) PRINTF_LIKE( 1, 2 );
void		VShowMessage( const char * format, va_list params ) PRINTF_LIKE( 1, 0 );
void		SaveMsgLog( const char * fileName );
void		ShowDump( const void * dataPtr, size_t count );

// Utilities_cl.cp
void		SimpleEncrypt( void * data, size_t size );
void		SpoolFramePicture( DataSpool * spool, DTSKeyID pictID, int horz, int vert );
void		UnspoolFramePicture( DataSpool * spool, DTSKeyID& pictID, int& horz, int& vert );

#if BITWISE_IMAGE_SPOOL
void		UnspoolFramePicture( DataSpool * spool, DTSKeyID& pictID, int& horz, int& vert,
								int idBits, int coordBits );
#endif  // BITWISE_IMAGE_SPOOL

DTSError	RemoveCrapFromFile( DTSKeyFile * file, const DTSKeyType * goodTypes );
DTSError	Delete1Type( DTSKeyFile * file, DTSKeyType type );
void		CompactName( const char * source, char * dest, size_t destsize );
DTSError	ReadFileVersion( DTSKeyFile * file, int32_t * version );

#if ! CL_SERVER
DTSError	RemoveCrapFromImagesFile( DTSKeyFile * file );
DTSError	RemoveCrapFromSoundsFile( DTSKeyFile * file );
DTSError	GetUnusedIDNotZero( DTSKeyFile * file, DTSKeyType inType, DTSKeyID& outID );
#endif  // ! CL_SERVER

#if defined( MULTILINGUAL )
int			ExtractLangText( const char ** text, int langid, int * gender );
#endif  // MULTILINGUAL

#endif  // PUBLIC_CL_H

