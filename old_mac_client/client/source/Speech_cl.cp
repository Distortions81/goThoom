/*
**	Speech_cl.cp		ClanLord
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

#include "Speech_cl.h"
#include "ClanLord.h"

/*
Theory of Operation:

When the app calls Speak( "..." ), we create a new queue item containing
a copy of the text (in a CFString), and add that to our local OS queue structure.

In our Idle routine, we look at the front of the queue to see whether
there's anything there, and, if so, whether the Speech Manager is
busy working on it or not. If there is and it isn't, we ask the Speech Mgr
to start speaking it. On the other hand, if there's an item but the SM 
has already digested it, we can dequeue it and dispose of it.

Our speech channel has a callback function installed which runs whenever
the Speech Mgr has completed processing a chunk of text (although it may
not have finished -- or possibly may not even have begun -- making the
_sounds_ requested.) When we get that callback, the Speech Mgr is ready to
accept a new text-to-speech request. So if there's a pending item on
the queue at that point, we mark it as in-progress, and feed it to the Speech
Mgr. This avoids unnecessarily stop-and-go jerky speech, if the requests arrive
rapidly, since the SM can be working on text-to-phonemes for one item even while
it's also working on phonemes-to-audio for the previous item.

[Except that those fancy completion-routine maneuvers turned out to be too ambitious;
they've been turned off for eons. Presently, the only job performed by the callback is
to change a queue item's state from 'in-progress' to 'finished'. All of the interesting
stuff now happens purely in the Idle routine, at task time, when it's safest.]
*/


// for development debugging of the speech queue
#define DEBUG_SPEECH		0 && defined( DEBUG_VERSION )


/*
**	Global variables
*/

bool	gSpeechAvailable;


/*
**	local definitions
*/

// SRefCon is not in the 10.4 SDK
#if MAC_OS_X_VERSION_MIN_REQUIRED < 1050
typedef SInt32 SRefCon;
#endif


// pending speech requests are stored in a queue, which is managed by
// the Queue functions of the Mac Toolbox. All that's encapsulated in
// a SpeechQueueItem.

#pragma pack( push, 2 )

class SpeechQueueItem
{
public:
	
	// OS QElems need a queueType; I picked this one at random
	static const short kSpeechQueueType = 129;
	
	enum SpeechQueueItemState
		{
		q_undefined,			// item has not yet been installed in queue
		q_pending,				// item is on queue, waiting for Speech Mgr
		q_inProgress,			// Speech Mgr is busy with this item
		q_finished,				// Speech Mgr has finished with item; safe to delete it
		q_trashable				// item has been delinked from the queue
		};
	
// these must be the first two fields, exactly as they are, to properly mimic an OSQueue
	SpeechQueueItem *		qLink;
	short					qType;
	
// everything after here is at our discretion
	CFStringRef				textString;	// the text to speak
	
	SpeechQueueItemState	state;		// pending, inProgress, finished...
	
	// constructor/destructor
				SpeechQueueItem( const char * text );	// a MacRoman C string
				~SpeechQueueItem();
	
	// queue manipulators
	void		EnqueueItem();
	OSStatus	DequeueItem();
	
	// accessors
	bool		IsPending()  const { return q_pending  == state; }
	bool		IsFinished() const { return q_finished == state; }
	
	// speak it!
	OSStatus	SpeakSelf();
	
private:
	// copy ctor and op=()
	// declared but not defined
				SpeechQueueItem( const SpeechQueueItem & );
	SpeechQueueItem& operator=( const SpeechQueueItem & );
};


// the head of the queue - again, this is essentially a simple QHdr
struct SpeechQueueHeader
{
	volatile short				qFlags;
	volatile SpeechQueueItem *	qHead;
	volatile SpeechQueueItem *	qTail;
};

#pragma pack( pop )


/*
**	internal globals
*/
static SpeechChannel 		gTheSpeechChannel;		// what we talk through
static SpeechQueueItem *	gCurrSpeechItem;		// currently-bespoken item
static bool					bSpeechSuspended;		// when true, ignore speech requests

static SpeechQueueHeader	gSpeechQueueHead;		// the header for the speech queue


/*
**	local routine prototypes
*/
static bool			IsSpeechAvailable();				// true if this machine has speech
static OSStatus		ReallyOpenSpeechChannel();			// lazy opener
static OSStatus		SetupSpeechChannel();				// prepare a speech channel
static void			CB_TextIsDone(						// the "text done" callback
						SpeechChannel	chan,
						SRefCon			refCon,
						const void **	nextBuf,
						ulong *			byteLen,
						SInt32 *		controlFlags );


/*
**	InitSpeech()
**
**	call once at startup to prepare the unit
**	this sets gSpeechAvailable
*/
void
InitSpeech()
{
	gSpeechAvailable = IsSpeechAvailable();
}


/*
**	SuspendSpeech()
**
**	call when the app is being suspended
**	eventually this might halt all speech and dispose of the channel
**	so that other processes can use the full speech bandwidth
*/
void
SuspendSpeech()
{
	bSpeechSuspended = true;
}


/*
**	ResumeSpeech()
**
**	call when the app is resumed
**	eventually this might undo the effects of SuspendSpeech()
*/
void
ResumeSpeech()
{
	bSpeechSuspended = false;
}


/*
**	IdleSpeech()
**
**	Called once per frame, by CLApp:Idle()
**	We dispose of any stale speech items in the queue,
**	& start speaking the first pending item in the queue, if any.
*/
void
IdleSpeech()
{
	if ( not gSpeechAvailable || not gTheSpeechChannel )
		return;
	
	// remove any stale items from front of queue
	OSStatus err;
	SpeechQueueItem * item = const_cast<SpeechQueueItem *>( gSpeechQueueHead.qHead );
	while ( item && item->IsFinished() )
		{
		err = item->DequeueItem();
		delete item;
		
		item = const_cast<SpeechQueueItem *>( gSpeechQueueHead.qHead );
		}
	
	// look for the first pending item that wasn't already scheduled by the callback
	// (but don't schedule it unless all previous speech is finished)
	if ( item
	&&   item->IsPending()
	&&   not SpeechBusy() )		// or SpeechBusySystemWide()?
		{
		item->SpeakSelf();
		}
}


#if DEBUG_SPEECH
/*
**	DumpSpeechQ()
**
**	print out all entries on the speech queue
*/
static void
DumpSpeechQ()
{
	fprintf( stderr, "Chan: %p\n", gTheSpeechChannel );
	if ( gTheSpeechChannel )
		{
		CFTypeRef tr = nullptr;
		fputs( "Status: ", stderr );
		CopySpeechProperty( gTheSpeechChannel, kSpeechStatusProperty, &tr );
		if ( tr )
			{
			CFShow( tr );
			CFRelease( tr );
			tr = nullptr;
			}
		else
			fputs( "none\n", stderr );
		
		fputs( "Errors: ", stderr );
		CopySpeechProperty( gTheSpeechChannel, kSpeechErrorsProperty, &tr );
		if ( tr )
			{
			CFShow( tr );
			CFRelease( tr );
			tr = nullptr;
			}
		else
			fputs( "none\n", stderr );
		}
	
	int ctr = 0;
	const SpeechQueueItem * item = const_cast<SpeechQueueItem *>( gSpeechQueueHead.qHead );
	while ( item )
		{
		char buff[ 256 ];
		if ( item->textString )
			CFStringGetCString( item->textString, buff, sizeof buff, kCFStringEncodingUTF8 );
		else
			strcpy( buff, "(null)" );
		
		const char * state;
		switch ( item->state )
			{
			case SpeechQueueItem::q_undefined:	state = "undef";	break;
			case SpeechQueueItem::q_pending:	state = "pending";	break;
			case SpeechQueueItem::q_inProgress:	state = "in progress";	break;
			case SpeechQueueItem::q_finished:	state = "finished";	break;
			case SpeechQueueItem::q_trashable:	state = "trash";	break;
			default:			state = "???";		break;
			}
		
		fprintf( stderr, "item #%d @ %p: text %s state %d %s\n",
			ctr++, item, buff, item->state, state );
		
		item = item->qLink;
		}
}
#endif  // DEBUG_SPEECH


/*
**	Speak()
**
**	add a new chunk of text to the queue
*/
DTSError
Speak( const char * text )
{
#if DEBUG_SPEECH
	if ( text && strstr( text, "report speech" ) )
		DumpSpeechQ();
#endif

	// safety check
	if ( not gSpeechAvailable )
		return noSynthFound;		// a vaguely relevant error code
	
	// no new speech in the background
	if ( bSpeechSuspended )
		return noErr;
	
	// well, we can't put off doing the lazy open any longer!
	DTSError result = ReallyOpenSpeechChannel();
	if ( noErr != result )
		return result;
	
	// allocate one new speech queue item
	SpeechQueueItem * item = NEW_TAG("SpeechQueueItem") SpeechQueueItem( text );
	if ( not item )
		result = memFullErr;
	
	// and link it into the queue
	if ( noErr == result )
		item->EnqueueItem();
	
	return result;
}


/*
**	ExitSpeech()
**
**	called at quit time
*/
void
ExitSpeech()
{
	if ( gTheSpeechChannel )
		{
		// shut up
		//
		// (possible fixme: when running the client in an emulated Snow Leopard server via
		// VMWare Fusion, there is no sound card, so speech calls never finish.
		// As a result, if we've made any calls to SpeakCFString(), this StopSpeech() here
		// will get stuck in an endless polling loop, and the app will hang.
		// we may need to put a time limit or something...
		//
		// ... or maybe we just don't call StopSpeech() at all? The app is exiting,
		// so the OS will do its own cleanup anyway)
		__Verify_noErr( StopSpeech( gTheSpeechChannel ) );
	
		// dispose of the speech channel
		__Verify_noErr( DisposeSpeechChannel( gTheSpeechChannel ) );
		
		gTheSpeechChannel = nullptr;
		}
	
	// dispose of the buffers
	while ( SpeechQueueItem * item = const_cast<SpeechQueueItem *>( gSpeechQueueHead.qHead ) )
		{
		__Verify_noErr( item->DequeueItem() );
		delete item;
		}
}


//	--------------------------------------------
//	local routines
//	--------------------------------------------

/*
**	IsSpeechAvailable()
**
**	check for presence of Speech Manager
*/
bool
IsSpeechAvailable()
{
	bool available = false;
	SInt32 data;
	OSStatus err = Gestalt( gestaltSpeechAttr, &data );
	if ( noErr == err )
		{
		available = ( (data & (1 << gestaltSpeechMgrPresent)) != 0 )
				&&  IsCFMResolved( SpeakCFString );
		}
	
	return available;
}


/*
**	ReallyOpenSpeechChannel()
**
**	this routine is called the first time someone actually
**	tries to _use_ the speech channel.
**	No sense opening the darned thing every startup if it's never used...
*/
OSStatus
ReallyOpenSpeechChannel()
{
	// only need to do this stuff once
	static bool bSpeechOpened;
	if ( bSpeechOpened )
		return noErr;
	bSpeechOpened = true;
	
	// and only if the machine _can_ talk
	OSStatus result = noErr;
	if ( gSpeechAvailable )
		{
		// set up the speech channel
		result = SetupSpeechChannel();
		
		// clear the queue header
		gSpeechQueueHead.qFlags = 0;
		gSpeechQueueHead.qHead = nullptr;
		gSpeechQueueHead.qTail = nullptr;
		
		// assume that we're frontmost
		bSpeechSuspended = false;
		}
	
	// if there was a speech error, pretend it doesn't exist
	if ( noErr != result )
		{
#if DEBUG_VERSION
		ShowMessage( BULLET " InitSpeech error: %d", (int) result );
#endif	// DEBUG_VERSION
		
		gSpeechAvailable = false;
		}
	
	return result;
}


// turn this on for newfangled speech properties
#define USE_SPEECH_PROPERTIES		1 && MAC_OS_X_VERSION_MIN_REQUIRED >= 1050


#if USE_SPEECH_PROPERTIES
/*
**	Set1SpeechProperty()
**
**	shim routine for setting a given speech channel property to
**	a particular 32-bit value (in fact, a pointer)
*/
static OSStatus
Set1SpeechProperty( SpeechChannel chan, CFStringRef property, const void * value )
{
	OSStatus err = coreFoundationUnknownErr;
	
	// even though we're setting void * values, we must wrap them up as CFNumbers (!)
//	__static_assert( sizeof(value) == sizeof(SInt32) );		// ergo this had better be true

	if ( CFNumberRef nr = CFNumberCreate( kCFAllocatorDefault, kCFNumberSInt32Type, &value ) )
		{
		err = SetSpeechProperty( chan, property, nr );
		CFRelease( nr );	// channel owns it now
		}
	
	return err;
}
#endif  // USE_SPEECH_PROPERTIES


/*
**	SetupSpeechChannel()
**
**	prepare the speech channel just the way we like it
*/
OSStatus
SetupSpeechChannel()
{
	// allocate the channel using default voice
	OSStatus err = NewSpeechChannel( nullptr, &gTheSpeechChannel );
	__Check_noErr( err );
	if ( noErr == err )
		{
		SpeechChannel chan = gTheSpeechChannel;
		
		// customize the channel with a refCon ...
		if ( noErr == err )
			{
#if USE_SPEECH_PROPERTIES
			err = Set1SpeechProperty( chan, kSpeechRefConProperty, &gCurrSpeechItem );
#else
			err = SetSpeechInfo( chan, soRefCon, &gCurrSpeechItem );
#endif
			__Check_noErr( err );
			}
		// ... and our callback
		if ( noErr == err )
			{
			SpeechTextDoneUPP upp = NewSpeechTextDoneUPP( CB_TextIsDone );
			const void * callbackAddr = reinterpret_cast<const void *>( upp );
			
#if USE_SPEECH_PROPERTIES
			err = Set1SpeechProperty( chan, kSpeechTextDoneCallBack, callbackAddr );
#else
			err = SetSpeechInfo( chan, soTextDoneCallBack, callbackAddr );
#endif
			__Check_noErr( err );
			}
		}
	
	return err;
}


//	--------------------------------------------
//	speech manager callback
//	--------------------------------------------

/*
**	CB_TextIsDone()
**
**	Speech Mgr has finished with the current queue item
*/
void
CB_TextIsDone( SpeechChannel	/* chan */,
				SRefCon			refCon,
				const void **	nextBuf,
				ulong *			byteLen,
				SInt32 *		/* controlFlags */ )
{
	// the refcon is actually a pointer to gCurrSpeechItem
	SpeechQueueItem ** pItem = reinterpret_cast<SpeechQueueItem **>( refCon );
	if ( not pItem || not *pItem )
		{
		// This shouldn't happen. But if it does, there's nothing more to be said
		// (if you'll pardon me for saying so...)
		*nextBuf = nullptr;
		*byteLen = 0;
		return;
		}
	
	// mark the item as finished
	SpeechQueueItem * item = *pItem;
	item->state = SpeechQueueItem::q_finished;
	
	// start in on the next item, if any.
	// v181: better yet, don't. Just leave it on the queue, and let
	// IdleSpeech() do all the juggling at task time.
	// This seems to work better with OS X, and may fix the bugs
	// folks have reported when lots of speakable items arrive in the same frame.

/*
	if ( item->qLink )
		{
		item = item->qLink;
		item->state = SpeechQueueItem::q_inProgress;
		
		// this tells the Speech Mgr to start work on the next chunk of text
		*nextBuf = item->textBuf;
		*byteLen = item->textLen;
		*controlFlags = 0;
		
		// also advance the gCurrSpeechItem to point to this new one
		*pItem = item;
		}
	else
		{ ... }
*/
	// tell Speech Mgr that we've no more text right now.
	*nextBuf = nullptr;
	*byteLen = 0;
	*pItem = nullptr;
}


/*
**	SpeechQueueItem implementation
*/


/*
**	SpeechQueueItem::SpeechQueueItem()
**	constructor
*/
SpeechQueueItem::SpeechQueueItem( const char * text ) :
		 qLink( nullptr ),
		 qType( kSpeechQueueType ),
		 textString( nullptr ),
		 state( q_undefined )
{
	if ( text )
		textString = CreateCFString( text );
}


/*
**	SpeechQueueItem::~SpeechQueueItem()
**	dtor
*/
SpeechQueueItem::~SpeechQueueItem()
{
	// sanity check
	__Check( (q_trashable == state) || (q_undefined == state) );
	
	if ( textString )
		CFRelease( textString );
}


/*
**	SpeechQueueItem::EnqueueItem()
**
**	append this to the tail of the queue
*/
void
SpeechQueueItem::EnqueueItem()
{
	// mark this item as not-yet-spoken
	state = q_pending;
	
	QHdrPtr queueHead = reinterpret_cast<QHdrPtr>( &gSpeechQueueHead );
	QElemPtr thisElem = reinterpret_cast<QElemPtr>( &qLink );
	
	Enqueue( thisElem, queueHead );
}


/*
**	SpeechQueueItem::DequeueItem()
**
**	remove this from the queue
*/
OSStatus
SpeechQueueItem::DequeueItem()
{
// we'd better be at the head of the queue, buckwheat
#ifdef DEBUG_VERSION
#define SpeechDebugFlag true

	if ( SpeechDebugFlag
	&&	 gSpeechQueueHead.qHead != reinterpret_cast<SpeechQueueItem *>( &qLink ) )
		{
		Beep();
		ShowMessage( "Dequeueing non-frontmost speech item!" );
		}
#endif	// DEBUG_VERSION

	// remove self from queue
	QHdrPtr queueHead = reinterpret_cast<QHdrPtr>( &gSpeechQueueHead );
	QElemPtr thisElem = reinterpret_cast<QElemPtr>( &qLink );

	OSStatus err = Dequeue( thisElem, queueHead );
	__Check_noErr( err );
	
	// mark this item as completed
	state = q_trashable;
	return err;
}


/*
**	SpeechQueueItem::SpeakSelf()
**
**	if this item is at the head of the queue, declaim it
*/
OSStatus
SpeechQueueItem::SpeakSelf()
{
	state = q_inProgress;
	gCurrSpeechItem = this;
	
	OSStatus err = SpeakCFString( gTheSpeechChannel, textString, nullptr );
	__Check_noErr( err );
	if ( noErr != err )
		{
		// if not speakable, just mark it as done
		state = q_finished;
		}
	
	return err;
}

