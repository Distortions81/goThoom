/*
**	KeychainUtils.cp		ClanLord
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

#include <Security/Security.h>

#include "Public_cl.h"
#include "KeychainUtils.h"

/*
namespace KeychainAccess
{
	// look up 'name' in keychain; fill out 'pw'
	OSStatus	Get( KCPassType kind, const char * name, char * pw, size_t len );
	
	// save 'name' / 'newPw' pair, overwriting any previous values
	OSStatus	Update( KCPassType kind, const char * name, const char * newPw );
	
	// like Update(), but assumes 'name' doesn't already exist in keychain
	OSStatus	Save( KCPassType kind, const char * name, const char * pw );
};
*/


// local routines
namespace {

/*
**	KCServiceName()
**
**	return description string for a given password kind,
**	which is either kAccountPassword or kCharacterPassword
*/
const char *
KCServiceName( KCPassType kind )
{
	const char * result = nullptr;
	
	switch ( kind )
		{
		case kPassType_Account:
			result = "Clan Lord Account";
			break;
		
		case kPassType_Character:
			result = "Clan Lord Character";
			break;
		}
	
	return result;
}


/*
**	CanonicalizeKCName()
**
**	basically just uppercase & UTF8-ify it
*/
static void
CanonicalizeKCName( const char * inName, size_t oNameLen, char * outName )
{
//	CompactName( inName, outName, oNameLen );
	assert( oNameLen > 0 );
	if ( CFMutableStringRef tmp = CFStringCreateMutable( kCFAllocatorDefault, kMaxNameLen ) )
		{
		CFStringAppendCString( tmp, inName, kCFStringEncodingMacRoman );
		CFLocaleRef locale = CFLocaleCopyCurrent();
		CFStringUppercase( tmp, locale );
		if ( locale )
			CFRelease( locale );
		CFStringGetCString( tmp, outName, oNameLen, kCFStringEncodingUTF8 );
		CFRelease( tmp );
		}
	else
		outName[ 0 ] = '\0';
}

}  //  anon namespace


namespace KeychainAccess
{

/*
**	Get()
**
**	Attempt to read a password from the main keychain
**	returns success/failure code. On success, *pw will contain
**	at most 'len' bytes of the password
*/
OSStatus
Get( KCPassType kind, const char * name, char * pw, size_t len )
{
	// canonicalize the player or account name
	char pname[ kMaxNameLen ];
	CanonicalizeKCName( name, sizeof pname, pname );
	
	// look it up in the keychain
	UInt32 length = 0;
	void * pwd = nullptr;
	
	const char * svcName = KCServiceName( kind );
	__Check( svcName );
	
	OSStatus result = SecKeychainFindGenericPassword( nullptr,
				strlen( svcName ), svcName,
				strlen( pname ), pname,
				&length, &pwd, nullptr );
	
	if ( noErr == result )
		{
		// convert 'pwd' buffer into a C string at 'pw'
		if ( length >= len - 1 )
			length = len - 1;
		memcpy( pw, pwd, length );
		pw[ length ] = '\0';
		}
	
	if ( pwd )
		__Verify_noErr( SecKeychainItemFreeContent( nullptr, pwd ) );
	
	// if it doesn't exist, that's (mostly) OK -- just return a soft error
	if ( errSecItemNotFound /* errKCItemNotFound */ == result )
		result = 1;
	
	return result;
}

/*
**	Update()
**
**	Saves a password into the main keychain.
**	'kind' controls whether this is a character or account password;
**	'name' is the account or char name;
**	'pw' is the password itself.
**	if 'pw' is empty, then any pre-existing password is deleted from the keychain.
**	Returns >0 for soft errors (e.g. no keychain) or an acceptable error code,
**	or noErr.
*/
OSStatus
Update( KCPassType kind, const char * name, const char * pw )
{
	const char * svcName = KCServiceName( kind );
	
	// canonicalize the player or account name
	char pname[ kMaxNameLen ];
	CanonicalizeKCName( name, sizeof pname, pname );
	
	// first look for a previous definition
	OSStatus result = noErr;
//	if ( noErr == result )
		{
		SecKeychainItemRef item = nullptr;
		void * oldBuffer = nullptr;
		UInt32 oldLen = 0;
		result = SecKeychainFindGenericPassword( nullptr,	// default chain
					strlen( svcName ), svcName,
//					strlen( name ), name,
					strlen( pname ), pname,
					&oldLen, &oldBuffer, &item );
		
		// if it existed, is it different?
		if ( noErr == result )
			{
			bool bDeleteIt = '\0' == *pw;
			bool bNoChange = not bDeleteIt &&
					(oldLen == strlen( pw )) &&
					0 == memcmp( oldBuffer, pw, oldLen );
			
			__Verify_noErr( SecKeychainItemFreeContent( nullptr, oldBuffer ) );
			
			// if they are the same (and we're not deleting) then we can go home early
			if ( not bDeleteIt && bNoChange )
				{
				CFRelease( item );
				return noErr;
				}
			
			// they differ: we must delete the old one (?)
			// [seems easier to add the updated one from scratch, rather than
			// updating the existing one]
			// also do this if we _want_ to delete it
			result = SecKeychainItemDelete( item );
			__Check_noErr( result );
			
			CFRelease( item );
			
			if ( bDeleteIt )
				return result;
			}
		
		// if it doesn't exist, that's OK
		else
		if ( errSecItemNotFound /* errKCItemNotFound */ == result )
			result = noErr;
		}
	
	// now add the new definition
	if ( noErr == result && pw[0] )
		{
		result = SecKeychainAddGenericPassword(
					nullptr,					// default keychain
					strlen( svcName ), svcName,	// service name
					strlen( pname ), pname,		// account name
					strlen( pw ), pw,			// password
					nullptr );					// don't need the item ref
		__Check_noErr( result );
		}
	
	return result;
}


/*
**	SavePassInKeychain()
**
**	a bit like the above, but simpler.
**	This is called only from Main_cl.cp, when migrating passwords
**	out of the client's Prefs file into the keychain, for the
**	first and only time.
*/
OSStatus
Save( KCPassType kind, const char * name, const char * pwd )
{
	// add the info directly to the chain.
	// We're assuming that there's no prior conflicting entry in the keychain,
	// because until v1061 we never kept anything there.
	const char * svcName = KCServiceName( kind );
	
	// canonicalize names
	char pname[ kMaxNameLen ];
	CanonicalizeKCName( name, sizeof pname, pname );
	
	// add it
	OSStatus result = SecKeychainAddGenericPassword(
						nullptr,						// default keychain
						strlen( svcName ), svcName,		// service name
						strlen( pname ), pname,			// account name (or char)
						strlen( pwd ), pwd,				// password
						nullptr );						// ignore item ref
	
	__Check_noErr( result );
	return result;
}


}
