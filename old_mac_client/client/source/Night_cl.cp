/*
**	Night_cl.cp		ClanLord
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
#include "Night_cl.h"
#include "Shadows_cl.h"


/*
**	NightInfo::Reset()
**
**	set to default values
*/
void
NightInfo::Reset()
{
	mBaseLevel		= 0;		// full daylight
	mAzimuth		= 90;		// sun due north, for south-pointing shadows
	mCloudy			= false;	// not cloudy
	mNightFlags		= 0;		// all off
	
	oldAzimuth = mAzimuth;
	redshift = 1.0f;			// no shift
	startOfTwilight = 0;		// dawn/dusk not currently in effect

	CalcCurLevel();
}


/*
**	NightInfo::CalcCurLevel()
**
**	calculate current "effective" night level,
**	given base level and modification flags
*/
void
NightInfo::CalcCurLevel() 
{
	int delta = 0;
	
	// apply modifications
	if ( mNightFlags & kLightNoNightMods )
		mCurLevel = 0;
	else
		{
		if ( mNightFlags & kLightAdjust25Pct )
			delta += 25;
		if ( mNightFlags & kLightAdjust50Pct )
			delta += 50;
		if ( mNightFlags & kLightAreaIsDarker )
			delta = -delta;
		
		mCurLevel = mBaseLevel - delta;
		}
	
	// keep in range
	if ( mCurLevel < 0 )
		mCurLevel = 0;
	else
	if ( mCurLevel > 100 )
		mCurLevel = 100;
	
	// apply shadow intensity modifications
	if ( mNightFlags & kLightNoShadows )
		mCurShadows = 0;
	else
		mCurShadows = 50 - mCurLevel;
	
	// keep in range
	if ( mCurShadows < 0 )
		mCurShadows = 0;
	
	// maybe apply clouds
	if ( mCloudy && mCurShadows > 25  )
		mCurShadows = 25;

#if 0
	// debug
	if ( gPlayingGame )
		ShowMessage( "Nght2: lv %d ang %d shd %d flg 0x%x",
			mCurLevel, mAzimuth, mCurShadows, mNightFlags );
#endif  // 0
}


//
//	Handy-dandy regular expression for parsing night messages
//
#define kNightRE "^/nt ([0-9]+) /sa ([-0-9]+) /cl ([01])"


/*
**	NightInfo::ScanServerMessage()
**
**	if this info text starts with the night prefix
**	then set the night and shadow effect globals
*/
bool
NightInfo::ScanServerMessage( const char * inCommand, size_t inLen )
{
	const char * p = inCommand;
	const char want[] = "/nt ";		// pre-v187
	const size_t wantLen = sizeof want - 1;
	
	if ( inLen <= wantLen )
		return false;
	
	static CHSRegExp re( kNightRE );
	if ( re.regexec( inCommand ) )
		{
		mBaseLevel		= std::atoi( re.mStartPtrs[ 1 ] );
		mAzimuth		= std::atoi( re.mStartPtrs[ 2 ] );
		mCloudy			= re.mStartPtrs[ 3 ][0] != '0';
		CalcCurLevel();
		return true;
		}
	
	int nightLevel, shadowLevel, sunAngle, declination;
	
	/*
	 *	stay compatible with v185 server
	 */
	if ( ! std::strncmp( p, want, wantLen ) )
		{
		p += wantLen;
		
		// first check for v100 night.spell data...
		int found = std::sscanf( p, "%d %d %d %d",
							&nightLevel,
							&shadowLevel,
							&sunAngle,
							&declination);
		
		// don't accept garbage
		if ( found >= 3 )
			{
#if 0
			// debug: compare requested values vs current
			ShowMessage( "Night: lv %d/%d ang %d/%d shd: %d/%d",
				nightLevel,		mCurLevel,
				sunAngle,		mAzimuth,
				shadowLevel,	mCurShadows );
#endif  // 0
			
			mCurLevel	= nightLevel;
			mAzimuth	= sunAngle;
			mCurShadows	= shadowLevel;
			return true;
			}
		
		// failing that, try for pre-v100 data (night level only, no shadow data)
		found = std::sscanf( p, "%d", &nightLevel );
		if ( found > 0 )
			{
			mCurLevel = nightLevel;
			return true;
			}
		}

	return false;
}


/*
**	NightInfo::SetShadows()
**
**	tell the shadow-caster what settings to use
*/
void
NightInfo::SetShadows() const
{
	SetShadowAngle( mCurShadows, mAzimuth, 0 );
}


void
NightInfo::CalculateMorningEveningRedshift()
{
	// known anomalies:
	// depending on what the default values are during login, the player
	// could log in during the middle of twilight, and either see no redshift,
	// or a partial redshift with an abrupt (discontinuous) end.
	// also, fast-forward during movies doesn't change the value of kTicksPerGameSecond,
	// which means the effect will terminate abruptly when it hits end-of-twilight
	// sooner than expected
	
		// The true value is hard to express; this is (currently) pretty close, though
	const float kTicksPerGameSecond = 60.0f / 4.09f;
		// 30 game-minutes * 60s/min * kTicksPerGameSecond
	const float kTwilightLengthInTicks = 30 * 60 * kTicksPerGameSecond;
	
#ifdef ARINDAL
	const float kMaxRedshift = 1.5f;	// testers complained this was a bit too much
#else
	const float kMaxRedshift = 1.25f;	// this might still be a bit too much, or a bit too little
#endif
	
	if ( oldAzimuth != mAzimuth )
		{
		if ( (oldAzimuth ==  -2 && mAzimuth ==  -1)		// start of dawn twilight
		||   (oldAzimuth == 179 && mAzimuth == 180) )		// start of dusk twilight
			{
			startOfTwilight = GetFrameCounter();
			}
		else
			startOfTwilight = 0;
		
		oldAzimuth = mAzimuth;
		}
	
		// defensive
	if ( mAzimuth != -1 && mAzimuth != 180 )	// whatever is neither dawn nor dusk
		startOfTwilight = 0;					// must be either full day or full night
	
		// redshift is piecewise linear, 1.0->kMaxRedshift for the first half
		// of twilight, then kMaxRedshift->1.0 for the second half
	if ( startOfTwilight )
		{
		float shift = float(GetFrameCounter() - startOfTwilight) / kTwilightLengthInTicks;
			// defensive
		if ( shift < 0.0f ) shift = 0.0f;
		if ( shift > 1.0f ) shift = 1.0f;
		
		if ( shift < 0.5f )
				// shift increasing from 0.0 to 0.5, redshift increases from 1.0 to kMaxRedshift
			redshift = 1.0f + shift * 2.0f * ( kMaxRedshift - 1.0f );
		else
				// shift increasing from 0.5 to 1.0, redshift decreases from kMaxRedshift to 1.0
			redshift = 1.0f + (1.0f - shift) * 2.0f * ( kMaxRedshift - 1.0f );
#if 0
		ShowMessage( "shift, redshift %f %f", shift, redshift );
#endif
		}
	else
		redshift = 1.0f;
#if 0
	redshift = 1.25f;
#endif
}


/*
**	NightInfo::GetEffectiveNightLimit()
**	basically a simple accessor for the max-night-percentage preference value,
**	except it also knows about the force-full-night flag in the draw-state header
**	illumination field
*/
int
NightInfo::GetEffectiveNightLimit() const
{
	int lim;
	if ( mNightFlags & kLightForce100Pct )
		lim = 100;
	else
		lim = gPrefsData.pdMaxNightPct;
	
	return lim;
}


#ifdef CL_DO_WEATHER 
/*
** Weather-Code, by torx@arindal.com
*/
void
WeatherInfo::Reset()
{
	mWeatherImg = 0;
	mOutdoorOnly = true;
}


/*
**	WeatherInfo::SetWeather()
*/
int
WeatherInfo::SetWeather( int image, bool outdooronly )
{
	mWeatherImg = image;
	mOutdoorOnly = outdooronly;	

	return image;
}
#endif // CL_DO_WEATHER
