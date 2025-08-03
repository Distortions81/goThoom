/*
**	Unique_cl.cp		ClanLord
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

#if 0
// this stuff is not currently used

/*
**	Entry Routines
*/
// void	RepelSnerts();


/*
**	RepelSnerts()
**
**	only allow one copy of the client per machine
*/
void
RepelSnerts()
{
	ProcessSerialNumber psn = { 0, kNoProcess };
	ProcessInfoRec info;
	Str255 buffer;
	
	info.processInfoLength = sizeof info;
	info.processName       = buffer;
	info.processAppSpec    = nullptr;
	
	uint count = 0;
	OSStatus result;
	for(;;)
		{
		result = ::GetNextProcess( &psn );
		if ( result )
			break;
		result = ::GetProcessInformation( &psn, &info );
		if ( result )
			break;
		if ( kClientAppSign == info.processSignature )
			++count;
		}
	
	// catch multiple users
	if ( count > 1 )
		{
		SetErrorCode( -1 );
		GenericError( "You may only run one copy of Clan Lord at a time." );
		return;
		}
	
	// also catch braniacs who try to alter the client's signature
	const ProcessSerialNumber usPsn = { 0, kCurrentProcess };
	result = ::GetProcessInformation( &usPsn, &info );
	if ( info.processSignature != kClientAppSign
	&&   info.processSignature != 'CLt\306' )	// escape hatch
		{
		SetErrorCode( -1 );
		GenericError( "You may only run one copy of Clan Lord at a time." );
		}
}

#endif	// 0

