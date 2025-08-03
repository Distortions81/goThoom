/*
**	KeychainUtils.h		ClanLord
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

#ifndef KEYCHAINUTILS_H
#define KEYCHAINUTILS_H


// we need to save both account- and character-level passwords
enum KCPassType
{
	kPassType_Account,
	kPassType_Character
};


namespace KeychainAccess
{
	// look up 'name' in keychain; fill out 'pw'
	OSStatus	Get( KCPassType kind, const char * name, char * pw, size_t len );
	
	// save 'name' / 'newPw' pair, overwriting any previous values
	OSStatus	Update( KCPassType kind, const char * name, const char * newPw );
	
	// like Update(), but assumes 'name' doesn't already exist in keychain
	OSStatus	Save( KCPassType kind, const char * name, const char * pw );
};


#endif  // KEYCHAINUTILS_H
