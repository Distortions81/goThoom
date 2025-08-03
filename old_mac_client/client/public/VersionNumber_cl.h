/*
**	VersionNumber.h		ClanLord, CLServer, CLEditor
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

#ifndef VERSIONNUMBER_H
#define VERSIONNUMBER_H

// these next two lines are the only ones that need to be modified by hand:
#define kBaseVersionNumber		1353
#define kSubVersionNumber		0

// That said, _please_ don't alter anything above, aside from the actual numeric values.
// Other parts of our toolchain need to parse the version numbers out of this file.
// In particular, the following awk(1) snippet must not fail, and must produce
// the right answer:
//		/^#define kBaseVersionNumber/ { print $3 }

#ifndef ARINDAL
# define kVersionNumber			kBaseVersionNumber
#else
	// Arindal's numbering scheme is ... peculiar.
# define kVersionBump			1
# define kVersionNumber			((100 * kBaseVersionNumber) + kVersionBump)
#endif  // ! ARINDAL

/*
	the rest of these definitions conspire to produce the following:
	
	kFullVersionNumber	an integer equal to (kVersionNumber << 8) + kSubVersionNumber
						Suitable for things like movie file versioning.
	
	kVersionTrailer		a string of the form ".n" where n is the sub-version.
						But if the sub-version is 0 then this string is just "".
						The existing code base requires this macro.
	
	kFullVersionString	a string of the form "nnn.m" where n is the whole version
						number and m is the sub-version number, or just "nnn" if m==0.
						Suitable for things like window titles and 'vers' resources.
*/

#define kFullVersionNumber ((kVersionNumber << 8) + kSubVersionNumber)

// some macro magic
#define stringify(x)	stringize1(x)
#define stringize1(x)	#x

// handle the (common) special case where kSubVersionNumber is 0
#if kSubVersionNumber
# define kVersionTrailer "." stringify( kSubVersionNumber )
#else
# define kVersionTrailer ""
#endif  // kSubVersionNumber

#ifndef ARINDAL
# define kFullVersionString stringify( kVersionNumber ) kVersionTrailer
#else
	// e.g. "517" "0" "1" ".2" -> "51701.2"
# define kFullVersionString \
	stringify( kBaseVersionNumber ) "0" stringify( kVersionBump ) kVersionTrailer
#endif  // ARINDAL

#endif  // VERSIONNUMBER_H
