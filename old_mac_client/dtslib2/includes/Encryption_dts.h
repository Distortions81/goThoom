/*
**	Encryption_dts.h		dtslib2
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

#ifndef Encryption_dts_h
#define Encryption_dts_h

#ifndef _dtslib2_
#include "Prefix_dts.h"
#endif


/*
**	Interface routines
*/
void		DTSOneWayHash( const void * source, size_t len, unsigned char hash[16] );

DTSError	DTSEncode( const void * plaintext, void * ciphertext, size_t len,
					   const void * key, size_t keylen );
DTSError	DTSDecode( const void * ciphertext, void * plaintext, size_t len,
					   const void * key, size_t keylen );


#endif // Encryption_dts_h
