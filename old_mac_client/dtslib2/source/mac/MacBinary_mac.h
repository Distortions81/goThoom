/*
**	MacBinary_mac.h		dtslib2
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

//################################################################################
//
//  This program is free software; you can redistribute it and/or
//  modify it under the terms of the GNU General Public License
//  as published by the Free Software Foundation; either version 2
//  of the License, or (at your option) any later version.
// 
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
// 
//  You should have received a copy of the GNU General Public License
//  along with this program; if not, write to the Free Software
//  Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA
//  02111-1307, USA.
//
//################################################################################


/* -------------------------------------------------------------------------------

	File:		MacBinary.h
	Authors:	Josef W. Wankerl
	
	Description:
		This file contains code to encoding/decode MacBinary I/II/III files.
		 
	Modification History:
		Apr. 10, 1999	Integrated into main MacCVSPro source.
		4/29/2001		swiped from MacCVSPro; adapted for clanlord
		7/8/2005		more of same
		6/2012			converted to FSRefs
-------------------------------------------------------------------------------- */


#ifndef MACBINARY_H
#define MACBINARY_H

#ifndef _dtslib2_
# include "Prefix_dts.h"
#endif

enum MacHeaderVersion
{
	kMacBinary,
	kMacBinaryII,
	kMacBinaryIII
};

const OSStatus kNotMacBinaryErr		= -208;		// aka badFileFormat

bool		IsValidMacBinary( const char * theHeader,  MacHeaderVersion * theVersion );

OSStatus	FSDecodeMacBinary( const FSRef * inTargetRef,
				OSType inCreator, OSType inType, ScriptCode inScript );

#endif	// MACBINARY_H
