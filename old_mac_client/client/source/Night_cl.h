/*
**	Night_cl.h		ClanLord
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

#ifndef NIGHT_CL_H
#define	NIGHT_CL_H


// enables client-side night/shadow effects
#define	CL_DO_NIGHT		1

// enables client-side weather-overlay images
#define CL_DO_WEATHER	1


enum NightPictures
	{
	kNight100Pct	= 1395,
	kNight75Pct,
	kNight50Pct,
	kNight25Pct
	};


class NightInfo
{
protected:
	// these fields are calculated by the client itself, based on the others
	int		mCurLevel;			// effective ('net') night level
	int		mCurShadows;		// effective shadow darkness
	
	// we get these fields from the timekeeper NPC. they change sidereally
	int		mBaseLevel;			//  0 -- 100 : ambient illumination
	int		mAzimuth;			// -1 -- 360 : sun's azimuth (0 = east, 90 = north)
	bool	mCloudy;			//  is it cloudy out? (make shadows dimmer)
	
	// these come from the frame headers. They may change on an area-by-area basis
	uint	mNightFlags;
	
	// the start, end, and progress of dawn/dusk are not sent by the server,
	// so we are going to try to fake it
	int		oldAzimuth;
	float redshift;				// this is a scale, typically 1.0 - 1.5 or so
	ulong startOfTwilight;		// in ticks
						// set when azimuth changes from 179 to 180 (dusk) or -2 to -1 (dawn)
						// twilight is over when 30 (IC) minutes have elapsed,
						// or azimuth = 181 or 0, whichever comes first

public:
	// interfaces
			NightInfo() { Reset(); }					// minimal ctor
	void	Reset();									// default settings
	void	CalcCurLevel();								// determine current effective light-level
	
	int		GetLevel() const							// getter
			{
#if 0
			// for debug: notify only if there's a change
			static int nightlevel = -1;
			static int azimuth = 90;
			if ( nightlevel != mCurLevel || azimuth != mAzimuth )
				{
				nightlevel = mCurLevel;
				azimuth = mAzimuth;
				ShowMessage( "night level = %d, mAzimuth = %d", nightlevel, azimuth );
				}
#endif  // 0
			
			// misc. fixed levels, for debugging - uncomment just ONE.
//			return 100;				// max night
//			return 50;				// "minimum" night
//			return 0;				// day
			return mCurLevel;		// real level
			}
		
	float	GetMorningEveningRedshift() const { return redshift; }
	void	CalculateMorningEveningRedshift();
	bool	ScanServerMessage( const char * msg, size_t len );	// decode messages from server
	void	SetShadows() const;									// tell the shadow-caster what's up
	void	SetFlags( uint newFlags )							// flag-watcher
				{
				if ( newFlags != mNightFlags )
					{
					mNightFlags = newFlags;
					CalcCurLevel();
					SetShadows();
					}
					// unfortunately, since this varies every frame,
					// in ways the flags can't flag,
					// we need to call it promiscuously
					// (otherwise it could just be part of CalcCurLevel() )
				CalculateMorningEveningRedshift();
				}
	int		GetEffectiveNightLimit() const;
};


#ifdef CL_DO_WEATHER

class WeatherInfo
{
protected:
	int		mWeatherImg;	// The image used to overlay
	bool	mOutdoorOnly;	// If set weather is only shown outdoors
	
public:
			WeatherInfo() { Reset(); }	// minimal ctor
	void	Reset();		// default settings
	int		GetImage() const								// getter
				{
				return mWeatherImg;
				}
	bool		GetOutdoorOnly() const							// getter
				{
				return mOutdoorOnly;
				}
	int		SetWeather( int image, bool outdooronly );

};
#endif // CL_DO_WEATHER

#endif	// NIGHT_CL_H

