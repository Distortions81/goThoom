/*
**	Prefix_mac.h		dtslib2
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

#ifndef Prefix_mac_h
#define Prefix_mac_h

/**********************************************************************/

// setup debugging symbols based on options
// these symbols may be temporarily juggled for development

#ifndef DTSLIB_DEBUG_BUILD
	// turn *OFF* general purpose debugging
# undef DEBUG_VERSION
	// turn *OFF* memory allocation block-checking via NEW_TAG
# undef DEBUG_VERSION_NEW
	// turn *OFF* memory allocation tag-tracing via NEW_TAG
# undef DEBUG_VERSION_TAGS
#else
	// turn *ON* general purpose debugging
# define DEBUG_VERSION		1
#endif  // ndef DTSLIB_DEBUG_BUILD

/**********************************************************************/

// setup everything else
// these symbols should not be juggled even temporarily for development

// DEBUG_VERSION_TAGS requires DEBUG_VERSION_NEW
#if defined( DEBUG_VERSION_TAGS ) && DEBUG_VERSION_TAGS
# ifndef DEBUG_VERSION_NEW
#  warning "DEBUG_VERSION_TAGS requires DEBUG_VERSION_NEW. Turning that on."
#  define DEBUG_VERSION_NEW	1
# endif
#endif

// we are building dtslib2
#define _dtslib2_

// on a macintosh
#define DTS_Mac

/**********************************************************************/

#endif // Prefix_Debug_mac_h
