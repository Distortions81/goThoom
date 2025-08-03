/*
**	Prefix_dts.h		dtslib2
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

#ifndef Prefix_dts_h
#define Prefix_dts_h

/*
**	check pre-defined symbols or symbols defined on the command line
**	to determine which platform we are running on...
**	and include the appropriate prefix file
*/

#if defined( __MWERKS__ ) && ! defined( __MACH__ )

/************************************************************************/
// Metrowerks

	#error "We assume you will be using pre-compiled header files."
	#error "ie a xxx.pch++ file that makes a :builds:xxx.mch file."
	
#elif defined( DTS_CLD_Linux )

/************************************************************************/
// Linux
//	this symbol should be defined on the command line.
//	ie jcc -DDTS_CLD_Linux

	#define DTS_BIG_ENDIAN 0
	#define DTS_LITTLE_ENDIAN 1
	
	#include "Prefix_linux.h"

#elif defined( DTS_CLD_Unix )

/************************************************************************/
// Sun Unix
//	this symbol should be defined on the command line.
//	ie gcc -DDTS_CLD_Unix

	#define DTS_BIG_ENDIAN 1
	#define DTS_LITTLE_ENDIAN 0
	
	#include "Prefix_unix.h"

#elif defined( DTS_CLD_Darwin )

/************************************************************************/
// Mac OS X Unix (aka Darwin, a flavor of BSD)
//	this symbol should be defined within your IDE, or on the command line.
//	ie cc -DDTS_CLD_Darwin

	#include "Prefix_darwin.h"

#elif defined( DTS_XCODE )

/************************************************************************/
// Mac OS X Carbon GUI

#include "Prefix_Mach.h"

#else

/************************************************************************/
// Unknown

	#error "Unknown Platform - please fix."

#endif


#endif	// Prefix_dts_h
