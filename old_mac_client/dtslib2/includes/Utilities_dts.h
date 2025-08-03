/*
**	Utilities_dts.h		dtslib2
**
**	Copyright 2023 Delta Tao Software, Inc.
** 
**	Licensed under the Apache License, Version 2.0 (the "License");
**	you may not use this file except in compliance with the License.
**	You may obtain a copy of the License at
** 
**		https://www.apache.org/licenses/LICENSE-2.0
** 
**	Unless required by applicable law or agreed to in writing, software
**	distributed under the License is distributed on an "AS IS" BASIS,
**	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
**	See the License for the specific language governing permissions and
**	imitations under the License.
*/

#ifndef Utilities_dts_h
#define Utilities_dts_h

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Local_dts.h"


/*
**	DTSDate class
**	note, this class doesn't refer to any particular timezone, so you can't
**	be sure whether it's based on UTC, or or local time, or even on "local non-DST" time.
**	The resemblance to the now-obsolete Mac DateTimeRec structure is not accidental,
**	but we no longer use that specific API.
*/
class DTSDate
{
public:
	ulong		dateInSeconds;
	int16_t		dateYear;			// 1904 to 2040
	int16_t		dateMonth;			// 1 to 12
	int16_t		dateDay;			// 1 to 31
	int16_t		dateHour;			// 0 to 23
	int16_t		dateMinute;			// 0 to 59
	int16_t		dateSecond;			// 0 to 59
	int16_t		dateDayOfWeek;		// 1 (monday) to 7 (sunday)
		// pre-carbon, encoding was:   1 (sunday) to 7 (saturday)
	
	// interface
	void	Get();
	void	SetFromYMDHMS();
	void	SetFromSeconds();
};


/*
**	DTSTimer class
**	replacement for Start/StopTimer() without the no-reentrancy limitation.
**	This class has a template argument "TimeVal" which specifies the type of the
**	underlying platform-specific time measurement unit.
**	In each platform implementation, you'll need to do something like..

		typedef CFAbsoluteTime					DTSTime_t;	// for Mac OS Carbon
			// ... or timeval, timespec, UnsignedWide, ulong, etc.
			// depending on what the platform provides.

		typedef DTSTimerPriv<DTSTime_t>			DTSTimer;

**	It's done with templates instead of our usual "DTSImplement" scheme because
**	the latter calls operator new() to create the "priv" implementation class,
**	which would introduce a non-deterministic element to the act of timing.
*/
template<class TimeVal>
class DTSTimerPriv
{
public:
					DTSTimerPriv();
	void			StartTimer();
	ulong			StopTimer() const;
	
private:
	static void		DiffTimer( TimeVal& diff, const TimeVal& endTime, const TimeVal& startTime );
	static ulong	ConvertToMicrosecs( const TimeVal& diff );
	static TimeVal	GetCurrentTime();
	
	TimeVal					mStartTime;
	
#ifdef DEBUG_VERSION
	bool					mTimerStarted;		// sanity check
#endif
};


/*
**	missing string utilities
*/
void	StringToUpper( char * str );	// this seems to be missing from mw libraries
void	StringToLower( char * str );	// this seems to be missing from mw libraries
char *	StringCopyDst( char * dst, const char * src );	// copy and return next destination byte


/*
**	There seems to be some confusion about how to use these routines.
**	Their purpose is to be able to copy c strings into buffers without overflowing
**	the destination buffer.
**	The typical usage is as follows:
**
**		char dest_buffer[ kBufferSize ];
**		const char * source_string = "some string";
**		StringCopySafe( dest_buffer, source_string, sizeof(dest_buffer) );
**
**	The typical error I'm seeing is to pass the length of the source_string as the 
**	third parameter.
**	This is silly for two reasons:
**	One, StringCopySafe already knows the length of the source string ie it stops
**	copying when it gets to the terminating null.
**	Two, what happens if the length of the string is greater than the size of the buffer?
**	Answer: StringCopySafe happily overflows the destination buffer.
**	This _completely_ defeats the purpose of the routine.
*/

	// copy, pad with zeroes, return next source byte
const char *	StringCopyPad(
					char *			dest_buffer,
					const char *	source_string,
					size_t			sizeof_dest_buffer );

	// copy without overflowing
#if HAVE_STRLCPY || TARGET_API_MAC_OSX
		// There's a libc version of this function, available on some platforms
	inline void StringCopySafe(
					char *			dest_buffer,
					const char *	source_string,
					size_t			sizeof_dest_buffer )
{
	(void) strlcpy( dest_buffer, source_string, sizeof_dest_buffer );
}
#else
		// otherwise we implement it ourselves
void			StringCopySafe(
					char *			dest_buffer,
					const char *	source_string,
					size_t			sizeof_dest_buffer );
#endif  // HAVE_STRLCPY || TARGET_API_MAC_OSX

/*
**	BufferToStringSafe copies an array of characters from source_buffer to dest_buffer until:
**	a) a null character is encountered in the source_buffer...
**	b) the end of the source_buffer is reached...
**	c) the end of the dest_buffer is reached...
**	a terminating null character is added to dest_buffer to make it a proper c string.
**	if the source_buffer does not fit in the dest_buffer the string will be truncated.
*/
void	BufferToStringSafe( char * dest_buffer, size_t sizeof_dest_buffer,
							const char * source_buffer, size_t source_buffer_length );

// snprintf is a more/less safe version of sprintf
// safe in that it won't overflow the buffer
// not perfectly safe because it returns the number of bytes that it would have written
// which could be larger than the size of the buffer
// so this could be bad:
//		len = snprintf( buff, sizeof(buff), "%s", str );
//		buff[len] = '\n';
//		buff[len+1] = 0;
// snprintfx and vsnprintfx pin the returned value to the one less than the size of the buffer
int		snprintfx( char * buff, size_t size, const char * format, ... ) PRINTF_LIKE( 3, 4 );
int		vsnprintfx( char * buff, size_t size, const char * fmt, va_list p ) PRINTF_LIKE( 3, 0 );


// C-to-Pascal string wrangling
// (should probably be in a new Utilities_mac.h or some such...
//	but who knows, they might turn out to be useful some day.)
#if defined( DTS_Mac )
#define	myctopstr( c, p )	::CopyCStringToPascal( (c), (p) );
#define	myptocstr( p, c )	::CopyPascalStringToC( (p), (c) );
#else
void	myctopstr( const char * cstr, unsigned char * pstr );
void	myptocstr( const unsigned char * pstr, char * cstr );
#endif	// DTSMac


/*
**	Utility Routines
*/
ulong	GetFrameCounter();						// return 60ths of a second since startup

ulong	GetRandom( ulong range );				// return random number between 0 and range - 1
ulong	GetRandSeed();							// get the random seed
void	SetRandSeed( ulong );					// set the random seed

DTSError GetAppData( uint32_t rType, int id, void ** oPtr, size_t * oSize );	// get a resource

int		GetNumberOfScreens();					// get the number of active screens.
void	GetMainScreen( DTSRect * bounds );		// get bounds of main screen
void	GetScreenBounds( int screenNumber, DTSRect * bounds );
												// get the size of a screen.

bool	IsButtonDown();							// true if mouse button is down
bool	IsButtonStillDown();					// true if button is still down from last click
bool	IsKeyDown( int key );					// true if the key is down

void	Beep();									// alert beep

#endif  // Utilities_dts_h

