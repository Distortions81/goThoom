/*
**	TuneHelper_cl.h
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

#ifndef TUNEHELPER_H
#define TUNEHELPER_H

#ifndef __QUICKTIMECOMPONENTS__
# include <QuickTime/QuickTime.h>
#endif

// Comment that out to remove the support for music commands!!!
#define		HANDLE_MUSICOMMANDS

// The format of the command handed by CTuneQueue::HandleCommand is as follows.
//	Note parameters in [] are optional; don't put any space anywhere.
//
//	/music/stop
//		Stop playing anything we had in store
// 	/music[/stop]/play[/tempo<value>][/inst<value>][/vol<value>][/part]/notes<music...>
//	+ Tempo value is between 60 and 180
//	+ Instrument value must be defined as in Cl Tune Helper popup, starting at zero
//	+ Volume value is in percentage from kDefaultVelocity
//	And for the multipart and multiparty:
//	+ /part (/P) for storing a song without playing, until the last one (with no /part) arrives
//	+ /with (/H) to tell this tune waits for another tune from a corresponding /W to arrive
//
//	Specifying a /stop before a /play stops the current song, if any.
//
// Alternate notation:
// 	/music[/S]/P[/T<value>][/I<value>][/V<value>][/M]/N<music...>


// general definitions
enum {
	kNumOctaves			= 3,			// allowable range
	kOctave				= 12,			// number of semitones per octave
	kDefaultVelocity	= 100,			// loudness 
	kMinimumTempo		= 60,			// in beats per minute
	kDefaultTempo		= 120,
	kMaximumTempo		= 180,
	kDefaultVoice		= 1,
	kMaxVoices			= 2
	};

//
// Small utility class
// to have a handle that I can add to as I want
//
class CDataHandle
{
public:
									CDataHandle();
	/* virtual */					~CDataHandle();
	
	char *							Add( const void * inData, long inSize );
	void							Reset();
#if 1
	Ptr								GetPtr() const		{ return reinterpret_cast<Ptr>(
															CFDataGetMutableBytePtr( mData ) ); }
#else
//	Handle							Get() const			{ return mData; }
	Ptr								GetPtr() const		{ return *mData; }
#endif
	
protected:
#if 1
	CFMutableDataRef				mData;
#else
	Handle							mData;
#endif
};


//
// everything below here is going to have be redone in terms of MusicTracks,
// MusicSequences, and MusicPlayers (from <AudioToolbox/MusicPlayer.h>).
//

#ifdef CL_TUNE_HELPER_APP
//
// This class handles the note player component
// of QuickTime Musical Instruments. Not needed by client.
//
class CQTMusic
{
public:
									CQTMusic();
	/* virtual */					~CQTMusic();
	
	ComponentResult					NewNoteChannel(
											short					inChannel,
											short					inPolyphonie );
	void							DisposeNoteChannel( short inVoice );
	
	OSStatus						PickInstrument( ToneDescription * outDescription );
	OSStatus						SetInstrument(
											short					inInstrument,
											ToneDescription *		outDescription );
	short							GetInstrument() const
		{
		return mInstrument.instrumentNumber;
		}
	
	void							PlayNote(
											short					inChannel,
											short					inNote,
											short					inVelocity );
	void							StopNote(
											short					inChannel,
											short					inNote );
	
protected:
	enum {
		max_Channels				= 2
	};
	short							mNumNc;
	ToneDescription					mInstrument;
	NoteAllocator					mNa;
	NoteRequest						mNoteRequest[ max_Channels ];
	NoteChannel						mNc[ max_Channels ];
};
#endif  // CL_TUNE_HELPER_APP


//
// An instrument definition, with its restrictions
//
class CCLInstrument 
{
public:
	enum InstFlags {
		flags_NoChords		= 0x01,
		flags_NoMelody		= 0x02,
		flags_LongChord		= 0x04,
		flags_Custom		= 0x80
	};
	
	static const int default_Polyphony	= 6;
	static const int bits_per_long		= sizeof(long) * 8;
	
	// a simple bitmap
	typedef ulong		SRestrictions[ ((kNumOctaves * kOctave) / bits_per_long) + 1 ];
	
	ToneDescription					mInstrument;				// QuickTime instrument data
	short							mOctaveOffset;				// Offset to kMiddleC in octave
#if DTS_BIG_ENDIAN
	ushort							mUnused 		: 4,
									mPolyphony	 	: 4,
									mFlags 			: 8;		// InstFlags
#else
	ushort							mPolyphony 		: 4,
									mUnused			: 4,
									mFlags 			: 8;		// InstFlags
#endif
	short							mChordVelocity;				// in PERCENTS
	short							mMelodyVelocity;			// in PERCENTS
	SRestrictions					mRestrictions;
	Str31							mName;						// CL Instrument Name
	
									CCLInstrument();
	
		// map from MIDI to mRestrictions numbering, and vice-versa
	short							NoteToOffset( short inNote ) const;
	short							OffsetToNote( short inOffset ) const;
	
	bool							HasMelody() const;
	bool							HasChords() const;
	short							GetPolyphony() const;
	bool							IsNoteRestricted( short inNote ) const;
	static bool						IsOffsetRestricted(
											const SRestrictions&	inRestrictions,
											short					inOffset );
#ifdef CL_TUNE_HELPER_APP
	void							SetNoteRestriction(
											short					inNote,
											bool					inRestrict );
	static void						SetOffsetRestriction(
											SRestrictions&			ioRestrictions,
											short					inOffset,
											bool					inRestrict );
#endif
};


//
// Class for handling the instrument-list resource
//
class CCLInstrumentList
{
public:
									CCLInstrumentList();
	/* virtual */					~CCLInstrumentList();
		
	static const OSType  			resType = 'clIn';
	static const SInt16				resID	= 128;
	
	int								Count() const
		{
		if ( mList )
			return CFDataGetLength( mList ) / sizeof(CCLInstrument);
		
		return -1;
		}
	
	OSStatus						Load( bool inCreate );
	short							Get( short inIndex, CCLInstrument & outInstrument ) const;
	
#ifdef CL_TUNE_HELPER_APP
	// Only for CL Tune Helper usage
	short							Save( short inIndex, CCLInstrument& inInstrument );
	short							Delete( short inIndex );
	short							BuildMenu( MenuRef inMenu, short inKeepItems );
#endif
	
protected:
	CFDataRef						mList;
};


//
// This class is the real text->music converter
//
class CTuneBuilder
{
public:
	// Pauses to add to a tune
	enum {
		flag_PauseStart			= 0x0001,
		flag_PauseEnd			= 0x0002
	};
	
	//
	//	Constructor, Destructor
	//

									CTuneBuilder();
	explicit						CTuneBuilder(
											CCLInstrument &			inInstrument,
											short					inTempo,
											short					inVelocity,
											short					inStartVoice = kDefaultVoice );
	/* virtual */					~CTuneBuilder() {};
	
	// Set the parameters without rebuilding the instance
	void							SetParameters(
											CCLInstrument &			inInstrument,
											short					inTempo,
											short					inVelocity );
										
	// Set the current tempo
	void							SetTempo( short					inTempo );

	// Set/Get the current instrument
	void							SetInstrument( CCLInstrument&	inInstrument );
	void							GetInstrument( CCLInstrument&	outInstrument ) const
		{
		outInstrument = mInstrument;
		}
	
	// Set the velocity
	void							SetVelocity( short					inVelocity );
	
	// Main function: parse text, build up the song, and return a potential error
	OSStatus						BuildTune(
											CDataHandle *			inSong,
											const char *			inTune,
											long					inMusicLen,
											bool					inErrors );

	
	// If an error occurred, return position in the text
	long							GetErrorPosition() const
		{
		return mErrorPosition;
		}
	
	OSStatus						MakeTuneHeader( CDataHandle * inHeader );

	long							GetTotalNotes() const
		{
		return mTotalNotes;
		}
	
	long							GetTotalChords() const
		{
		return mTotalChords;
		}
	
	// return the duration in beats (p1 = 1 beat).
	// Can be musically wrong if tempo changes occur.
	ulong							GetDurationBeats() const
		{
		return mBeat;
		}
	
	// returns duration in QT 1/600s
	ulong							GetDuration() const
		{
		return mTotalDuration;
		}
	
#ifdef CL_TUNE_HELPER_APP
	// Play the tune until it ends, or you click.
	// it's used by the CL Tune Helper, not the client.
	OSStatus						SimplePlayTune(
											CDataHandle *			inTune,
											TunePlayCallBackUPP		inCallback = nullptr,
											long					inRefCon = nullptr );	
#endif
	
	enum {
		duration_Black				= 2,
		duration_White				= 4
	};
	enum {
		ratio_NoteRest				= 90,		// 90% of rest duration for the note
		max_NotesInChord			= 12,
		max_Marks					= 6,
		max_Endings					= 10,
		default_ChordDuration		= 4
	};
	
	// Error values
	enum {
		error_None					= 0,
		error_InvalidNote,
		error_InvalidModifier,
		error_ToneOverflow,
		error_PolyphonyOverflow,
		error_InvalidOctave,
		error_InvalidChord,
		error_UnsupportedInstrument,
		error_OutOfMemory,
		error_TooManyMarks,
		error_UnmatchedMark,
		error_UnmatchedComment,
		error_InvalidTempo,
		error_InvalidTempoChange,
		error_ModifierNeedValue,
		error_DuplicateEnding,
		error_DuplicateDefEnding,
		error_EndingInChord,
		error_EndingOutsideLoop,
		error_DefaultEndingError,
		error_InvalidEndingIndex,
		error_unterminatedLoop,
		error_unterminatedChord,
		error_AlreadyPlaying,		// actually used by CTuneQueue, but more convenient here
		error_Max
	};

	// Error messages
	static const char * const		sErrorMessages[];
		
#ifdef CL_TUNE_HELPER_APP
	// Flags types for the text. used by coloring.
	enum {
		type_Note					= 	0x01,
		type_Modifier				=	0x02,
		type_Comment				=	0x04,
		type_InChord				=	0x08,
		type_OctaveMark				= 	0x10,
		type_LoopMark				=	0x20,
		type_Pause					=	0x40,
		type_Error					=	0x80
	};
	
	struct SNote2Text
	{
		ulong						mTextOffset		: 16,		// Offset in text
									mNote			: 8,		// Note
									mLength			: 4,		// in character
									mVoice			: 4;		// QuickTime voice
	};
#endif  // CL_TUNE_HELPER_APP
	
protected:
	// Various Parameters
	CCLInstrument					mInstrument;
	short							mMelodyVoice;				// MIDI voice #
	short							mChordVoice;				// Chord voice #
	short							mBaseNote;
	short							mMinNote;
	short							mMaxNote;
	short							mTempo;
	short							mNoteLength;
	short							mPauseLength;
	short							mVelocity;
	// These are the maximum velocities from 0 to 100%
	short							mChordVelocity;
	short							mMelodyVelocity;
	// These are the current velocities, from 1 to 10 (10% to 100% of the previous values)
	short							mChordVolume;
	short							mMelodyVolume;
	
	//
	// State machine states (mStatus) for BuildTune()
	//
	enum TBStatus
		{
		status_PickNote 			= 0,
		status_HasNote,
		status_PickModifiers,
		status_HasModifiers,
		status_PickTempo,
		status_HasTempo,
		status_PickVolume,
		status_HasChord,
		status_HasPause,
		status_HasMark,
		status_HasEndMark,
		status_HasEnding,
		status_HasDefEnding,
		status_StuffNote,
		status_StuffPause,
		status_StuffChord
		};

	// State for the state machine for BuildTune()
	TBStatus						mStatus;
	long							mBeat;
	char							mCommandChar;
	
	// Note related
	short							mOctave;
	short							mNote;
	short							mDuration;
	bool							mLinked;
	
	// Chord related
	short							mChord[ max_NotesInChord ];
	short							mChordNum;
	short							mChordDuration;
	bool							mBuildingChord;
	
	// The chord being currently played. It will 'age' using the beat numbers
	struct SChord
		{
		short						note;			// note being played
		short 						duration;		// in 'beats'
		long						startBeat;		// beat this one started in
		short						velocity;		// at start of note
		ulong						event;			// offset to the event where that note starts
		};
	SChord							mCurrentChord[ max_NotesInChord ];
	short							mCurrentChordNum;
	short							mMaxChordPolyphony;	// maximum notes played on chord voice
	
	// Pause related
	short							mPauseDuration;
	
	bool							mSilent;			// Do not stuff notes
	
	// Mark related
	struct SMark
		{
		const char *				pos;
		short						count;
		const char *				end;				// end of loop
		short						index;
		const char *				ending[ max_Endings ];
		const char *				endingDefault;
		short						endingCurrent;
		bool						silentState;
		};
	SMark							mMark[ max_Marks ];
	short							mMarkNum;
	
	// Comment nesting count
	short							mComment;
	// used for tempo changes
	short							mNewTempo;
	// Error related
	OSStatus						mError;
	long							mErrorPosition;
	
	// Stats related
	long							mTotalNotes;
	long							mTotalChords;
	ulong							mTotalDuration;
	
#ifdef CL_TUNE_HELPER_APP
	//
	// Note (music) to text mapping handle
	// is optional, as well
	//
	CDataHandle *					mNote2Text;
	short							mNoteTextOffset;
	short							mNoteTextLen;
#endif
		
	//
	// tests for various categories of notation
	//
	static bool						IsSkip( char c )
		{	return (c == ' ' || c == '\r' || c == '\n' || c == '\t');	}
	
	static bool						IsComment( char c, bool start )
		{	return start ? c == '<' : c == '>'; 	}
	
	static bool						IsMark( char c, bool start )
		{	return start ? c == '(' : c == ')';	}
	
	static bool						IsChord( char c, bool start )
		{	return start ? c == '[' : c == ']';	}
	
	static bool						IsPause( char c )
		{	return c == 'p';	}
	
	static bool						IsDuration( char c )
		{	return (c >= '1' && c <= '9');	}
	
	static bool						IsNoteModifier( char c )
		{	return c == '#' || c == '.' || c == '_' || IsDuration(c);	}
	
	static bool						IsOctave( char c )
		{	return c == '+' || c == '-' || c == '=' || c == '/' || c == '\\';	}
	
	static bool						IsNote( char c )
		{	return (c >= 'a' && c <= 'g') || (c >= 'A' && c <= 'G'); 	}
	
	static bool						IsLongChord( char c )
		{	return c == '$'; }
	
	static bool						IsTempoChange( char c )
		{	return c == '@'; }
	
	static bool						IsVolume( char c )
		{	return c == '%' || c == '{' || c == '}';	}
	
	static char						IsEnding( char c )
		{	return c == '|' || c == '!' ? c : 0; }
	
	//
	// Methods to actually analyse the characters
	//
	static short					GetDuration( char c )	{ return c - '0'; }
	bool							GetNote( char c );
	OSStatus						GetNoteModifier( char c );
	OSStatus						GetOctave( char c );

	//
	// Increment the beat number
	//
	void							IncrementBeat( short			inDuration );
	void							LongChordFinishNote(
											CDataHandle *			inSong,
											short					inIndex );
	void							LongChordFinish( CDataHandle *	inSong );
	
	//
	// These do the stuffing into the mSong handle
	// they are called by the BuildTune()
	//
	
	static MusicOpWord *			GrowSong(
											CDataHandle *			inSong,
											int						inOpsNeeded );
	
	OSStatus						StuffNote(
											CDataHandle *			inSong,
											short					inVoice,
											short					inNote,
											bool					inLinked,
											short					inDuration );
	OSStatus						StuffChord(
											CDataHandle *			inSong,
											short					inVoice,
											const short *			inChord,
											short					inChordLen,
											bool					inLinked,
											short					inDuration );
	OSStatus						StuffPause(
											CDataHandle *			inSong,
											short					inVoice,
											short					inDuration );
	OSStatus						StuffEndSong( CDataHandle *		inSong );
	OSStatus						StuffNoteRequest(
											CDataHandle *			inSong,
											short					inVoice,
											const NoteRequest *		inNote );
#ifdef CL_TUNE_HELPER_APP
	void							StuffNoteOffet(
											short					inNote,
											short					inVoice );
#endif  // CL_TUNE_HELPER_APP
	
	// Returns a QT Duration from a CLTH Duration
	short							StuffGetNoteDuration(
											bool					inLinked,
											short					inDuration ) const;
};


//
// An asynchronous tune player
//
// Note: Given the 2 CDataHandles, the object will delete them itself, once finished!
//
class CTunePlayer
{
public:
	enum {
		max_Parts			= 5,
		part_Timeout		= 20,			// delete after 20 seconds without parts
		max_Syncs			= 3				// 3 players at the same time
	};
	
									CTunePlayer(
											ulong					inID,
											CDataHandle *			inData,
											CDataHandle *			inSong,
											const CDataHandle *		inHeader );
	/* virtual */					~CTunePlayer();
	
	OSStatus						Prepare();
	OSStatus						Play();
	bool							IsPlaying() const;
	
	OSStatus						QueueTune( const char * inTune, long inLen, bool bLastPart );
	
	ulong							GetID() const { return mID; }
	long							GetBufferLen() const { return mTuneBufferLen; }
	const char *					GetBufferPtr() const { return mTuneBuffer->GetPtr(); }
	CDataHandle *					GetCompiledTune() const { return mTune; }
	
	void							GetParameters(
											short &					outInstrument,
											short &					outTempo,
											short &					outVelocity ) const;
	void							SetParameters(
											short					inInstrument,
											short					inTempo,
											short					inVelocity );
	void							Touch() { mTouched = GetCurrentEventTime(); }
	bool							Timeout() const;
	bool							Played() const { return mPlayed; }
	bool							Purgeable() const;
	void							ChangeVolume( int inPercent );
	
protected:
	ulong					mID;					// This tune ID
	EventTime				mTouched;				// Last time we received a part
	short					mInstrument;			// Instrument index
	short					mTempo;					// Tempo
	short					mVelocity;				// Velocity, in percent
	TunePlayer				mPlayer;				// Actual QuickTime Component
	const CDataHandle *		mHeader;				// Tune Header, unique
	short					mTunesNum;				// Number of part stored
	short					mTunesQueued;			// Number of parts actually QT Queued
	CDataHandle *			mTuneBuffer;			// Tune data (uncompiled)
	long					mTuneBufferLen;			// Size of tune data buffer
	CDataHandle *			mTune;					// Tune data (compiled)
	bool					mPlayed;				// This tune has been played, or is playing
};


//
// This class helps synchronize various people playing together.
// The songs all play simultaneously, as soon as everyone has sent their own part of the whole
//
class CTuneSync
{
public:
									CTuneSync()		{ Reset(); }
	/* virtual */					~CTuneSync()	{}
	
	enum {
		max_Sync			= 3		// must be less than CHAR_BIT
									// otherwise, must enlarge mWant and mGot
	};
	
	// When we want a particular ID, we call this
	bool							Want( ulong inID );
	
	// When we receive a particular ID, we call this
	void							Got( ulong inID );
	
	// When we want to know if an ID is present (has been received) or not
	bool							Have( ulong inID ) const;
	
	// If we have Got()ten all we Want()ed
	bool							Ready() const 	{ return mWant && mWant == mGot; }
	
	// To reset the bitfields
	void							Reset()		{ mWant = mGot = 0; }
	
protected:
	uchar							mWant;		// bitmap
	uchar							mGot;		// bitmap
	ulong							mIDs[ max_Sync ];
};


//
//	CTuneQueue
//	This is the only static object you might have in the application
//
class CTuneQueue
{
public:
									CTuneQueue();
	/* virtual */					~CTuneQueue();

	bool							HandleCommand( const char * inCommand, long inLen );

	bool							IsPlaying( ulong inPlayerID = 0 ) const;
	void							StopPlaying( ulong inPlayerID = 0 );
	void							Idle();
	void							ChangeVolume( int vol );	// master volume

#ifdef CL_TUNE_HELPER_APP
	// Only for CL Tune Helper usage
	void							Load()		{ mInstruments.Load( false ); }
#endif
	
protected:
	enum {
		max_Players			= 6,
		system_Queue		= 0,	// the anonymous music queue
		players_Queue				// Index of first player queue
	};
	
	CCLInstrumentList				mInstruments;
	CTuneBuilder					mBuilder;
	CTunePlayer *					mPlayers[ max_Players ];
	CTuneSync						mSync;
	
	//
	// Command lookup number utilities
	//
	enum {
		cmd_Bad				= 0,
		cmd_Music,
		cmd_Play,
		cmd_Stop,
		cmd_Who,
		cmd_Tempo,
		cmd_Instrument,
		cmd_Volume,
		cmd_Part,
		cmd_With,
		cmd_Me,
		cmd_Notes
	};
	
	struct STextCmd
	{
		const char *	tcmd;
		short			cmd;
	};
	
	static const STextCmd			sCommands[];
	
	short							LookupCommand( const char *& p ) const;
	bool							GetPlayer( ulong inPlayerID, short& outPlayerIndex ) const;
	OSStatus						QueueTune(
											ulong					inPlayerID,
											const char *			inTune,
											long					inLen,
											bool					inErrors );
};

#endif	// TUNEHELPER_H

