/*
**	Scrap_mac.cp		dtslib2
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

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

#include "Scrap_dts.h"


/*
**	Entry Routines
**
**	ClearScrap();
**	CopyToScrap();
**	GetScrapSize();
**	GetFromScrap();
*/


/*
**	ClearScrap()
*/
DTSError
ClearScrap()
{
	OSStatus result = ::ClearCurrentScrap();
	__Check_noErr( result );
	return result;
}


/*
**	CopyToScrap()
**
**	copy the data to the scrap (or clipboard)
*/
DTSError
CopyToScrap( ulong stype, const void * ptr, size_t sz )
{
	ScrapRef scrap;
	OSStatus result = ::GetCurrentScrap( &scrap );
	__Check_noErr( result );
	if ( noErr == result )
		{
		result = ::PutScrapFlavor( scrap, stype, 0, sz, ptr );
		__Check_noErr( result );
		}
	return result;
}


/*
**	GetScrapSize()
**
**	return the size of the data in the scrap (or clipboard)
**	returns 0 on error (which might be considered an interface bug)
*/
size_t
GetScrapSize( ulong stype )
{
	ScrapRef scrap;
	::Size sz;
	OSStatus result = ::GetCurrentScrap( &scrap );
	__Check_noErr( result );
	if ( noErr == result )
		{
		result = ::GetScrapFlavorSize( scrap, stype, &sz );
		// skip next check, since a noTypeErr, e.g., is possible (and acceptable)
		// __Check_noErr( result );
		}
	if ( result != noErr )
		sz = 0;
	
	return sz;
}


/*
**	GetFromScrap()
**
**	copy the data in the scrap to the pointer
**	assume there is enough room, unless destSize is specified
*/
DTSError
GetFromScrap( ulong stype, void * ptr, size_t destSize /* = 0 */ )
{
	ScrapRef scrap;
	SInt32 sz;
	DTSError result = ::GetCurrentScrap( &scrap );
	__Check_noErr( result );
	if ( noErr == result )
		{
		result = ::GetScrapFlavorSize( scrap, stype, &sz );
//		__Check_noErr( result );	// -- might get noTypeErr, etc.
		}
	if ( noErr == result )
		{
		// clamp to desired size
		if ( destSize && destSize < uint( sz ) )
			sz = destSize;
		result = ::GetScrapFlavorData( scrap, stype, &sz, ptr );
		__Check_noErr( result );
		}
	
	SetErrorCode( result );		// fixme: eliminate this; callers should check result instead
	return result;
}

