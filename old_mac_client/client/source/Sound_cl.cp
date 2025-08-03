/*
**	Sound_cl.cp		ClanLord
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


/*
**	Entry Routines
*/
/*
void	CLInitSound();
void	CLExitSound();
void	CLPlaySound( DTSKeyID sndID );
void	SetSoundVolume( uint volume );
*/


/*
**	Internal Definitions
*/
const int kNumTracks = 4;				// max # simultaneous channels


/*
**	class CLSound
**
**	originally borrowed from Dark Castle
*/
class CLSound : public CacheObject
{
public:
	enum SoundPriority
		{
		kPriorityLowest,	// only used when sounds duplicated
//		kPriorityLow,		// rat squeak sound	(CL unused)
		kPriorityNormal,	// everything else
//		kPriorityHigh,		// monkey warn, dragon breath (CL unused)
//		kPriorityContinuous	// shield, waterfall (CL unused)
		};
	
	DTSKeyID		soundID;
	SoundPriority	soundPriority;
	DTSSound		soundDTS;
	
	// constructor/destructor
				CLSound() : CacheObject( kCacheTypeSound )	{}
	virtual		~CLSound();
	
	// interface
	DTSError	Init( DTSKeyID sndID, SoundPriority priority );
	void		Play();
	bool		IsDone() const { return soundDTS.IsDone(); }
	void		Stop() { soundDTS.Stop(); }
	
	// overloaded routines
	virtual bool CanBeDeleted() const { return IsDone(); }
};


/*
**	struct CLTrack
**
**	originally borrowed from Dark Castle
*/
struct CLTrack
{
	CLSound *		trackSound;
	int				trackPriority;
	
	// interface
	void	Play( CLSound * sound );
};


/*
**	Internal Routines
*/
static CLSound *	CacheSound( DTSKeyID sndID );


/*
**	Internal Variables
*/
static CLTrack			gTrack[ kNumTracks ];



/*
**	CLInitSound()
**
**	Initialize the sound channels.
*/
void
CLInitSound()
{
	DTSInitSound( kNumTracks );
	
	// set the baseline volume
	SetSoundVolume( gPrefsData.pdSoundVolume );
}


/*
**	CLExitSound()
**
**	Dispose of the sound channels.
*/
void
CLExitSound()
{
	DTSExitSound();
}


/*
**	CLPlaySound()
**
**	play a sound.
*/
void
CLPlaySound( DTSKeyID sndID )
{
	if ( not gPrefsData.pdSound )
		return;
	
	if ( CLSound * sound = CacheSound( sndID ) )
		sound->Play();
	
	// clear the error code, because we don't care if there was an error
	// but someone later might get confused by our leftover error
	ClearErrorCode();
}



/*
**	SetSoundVolume()
**
**	set the sound level
*/
void
SetSoundVolume( uint volume )
{
	// scale from a percentage to the correct range (0 .. 0xFFFF)
	if ( volume > 100 )
		volume = 100;
	volume *= 0x0FFFF / 100;
	DTSSetSoundVolume( volume );
}



/*
**	CLSound::~CLSound()
*/
CLSound::~CLSound()
{
	// subtract ourselves from all tracks
	for ( int ii = 0; ii < kNumTracks; ++ii )
		{
		if ( gTrack[ ii ].trackSound == this )
			gTrack[ ii ].trackSound = nullptr;
		}
}


/*
**	CLSound::Init();
*/
DTSError
CLSound::Init( DTSKeyID sndID, SoundPriority priority )
{
	void * data = nullptr;
	size_t len = 0;
	DTSError result = TaggedReadAlloc( &gClientSoundsFile, kTypeSound,
							sndID, data, "CLSoundInitSnd" );
	if ( noErr == result )
		result = gClientSoundsFile.GetSize( kTypeSound, sndID, &len );
	if ( noErr == result )
		result = soundDTS.Init( data, len );
	if ( noErr == result )
		soundPriority = priority;
	
	// now that the DTSSound has the bytes (and has copied them to its internal storage)
	// we can safely get rid of our originals, which are redundant & wasteful
	delete[] static_cast<char *>( data );	// from TaggedReadAlloc()
	
	return result;
}


/*
**	CLSound::Play()
*/
void
CLSound::Play()
{
	// see what tracks are still playing
	CLTrack * track = gTrack;	// i.e., &gTrack[ 0 ]
	int nnn;
	for ( nnn = kNumTracks - 1;  nnn >= 0;  --nnn, ++track )
		{
		if ( const CLSound * sound = track->trackSound )
			{
			if ( sound->soundDTS.IsDone() )
				track->trackSound = nullptr;
			}
		}
	
	// lower the priority of all tracks already playing this sound
	track = gTrack;
	for ( nnn = kNumTracks - 1;  nnn >= 0;  --nnn, ++track )
		{
		if ( track->trackSound == this )
			track->trackPriority = kPriorityLowest;
		}
	
	// find a track that is not playing
	// if there is one, start playing the sound, and we're done.
	track = gTrack;
	for ( nnn = kNumTracks - 1;  nnn >= 0;  --nnn, ++track )
		{
		if ( not track->trackSound )
			{
			track->Play( this );
			return;
			}
		}
	
	// No tracks were idle.
	// find the track with the lowest priority...
	int lowestPriority = INT_MAX;
	CLTrack * lowestTrack = nullptr;
	track = gTrack;
	for ( nnn = kNumTracks - 1;  nnn >= 0;  --nnn, ++track )
		{
		int priority = track->trackPriority;
		if ( lowestPriority > priority )
			{
			lowestPriority = priority;
			lowestTrack    = track;
			}
		}
	
	// ... and stop that track; then play this sound in its place
	if ( lowestTrack )
		{
		lowestTrack->trackSound->soundDTS.Stop();
		lowestTrack->Play( this );
		}
}


/*
**	CLTrack::Play()
**
**	assign a sound to a track, and start it playing
*/
void
CLTrack::Play( CLSound * sound )
{
	trackSound    = sound;
	trackPriority = sound->soundPriority;
	sound->soundDTS.Play();
}


/*
**	LoadCacheSound()
**
**	load the sound
*/
static DTSError
LoadCacheSound( DTSKeyID sndID, CLSound ** psound )
{
	CLSound * sound = nullptr;
	
	// allocate a sound record
	DTSError result = noErr;
//	if ( noErr == result )
		{
		sound = NEW_TAG("CLSound") CLSound;
		if ( not sound )
			result = memFullErr;
		}
	// load the sound
	if ( noErr == result )
		result = sound->Init( sndID, CLSound::kPriorityNormal );
	
	// recover from an error
	if ( result != noErr )
		{
		delete sound;
		sound = nullptr;
		}
	
	// return the sound record and the result
	if ( psound )
		*psound = sound;
	
	return result;
}


/*
**	CacheSound()
**
**	find the sound in the linked list, or add it
*/
CLSound *
CacheSound( DTSKeyID sndID )
{
	// search for it in the sound cache
	for ( CacheObject * walk = gRootCacheObject;  walk;  walk = walk->linkNext )
		{
		if ( CacheObject::kCacheTypeSound == walk->coType )
			{
			CLSound * sound = static_cast<CLSound *>( walk );
			
			// we found it!
			if ( sound->soundID == sndID )
				{
				// mark it as most-recently-used
				sound->Touch();
				return sound;
				}
			}
		}
	
	// it was not in the cache
	CLSound * sound = nullptr;
	
	// load the data and deal with memory issues
	DTSError result = LoadCacheSound( sndID, &sound );
	
	// cache the sound
	if ( noErr == result )
		{
		sound->soundID = sndID;
		sound->InstallLast( gRootCacheObject );
		}
	
	return sound;
}


