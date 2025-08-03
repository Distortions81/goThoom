/*
**	KeyFile_dts.h		dtslib2
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

#ifndef KeyFile_dts_h
#define KeyFile_dts_h

#ifndef _dtslib2_
#include "Prefix_dts.h"
#endif

#include "Local_dts.h"
#include "Platform_dts.h"


/*
**	Define the DTSKeyFile class
**
**	DTSKeyFile creates the file and opens it for writing if bWritePerm is true.
**	DTSKeyFile opens the file for reading if bWritePerm is false.
**	Read reads the record with the type and id.
**	Write writes a record with the type and id.
**	Delete removes the record with the type and id.
**	Compress compresses the file so there is no wasted space, also the records
**		are stored in the file in the same order as they were added.
*/
typedef int32_t DTSKeyType;
typedef int32_t DTSKeyID;

struct DTSKeyInfo
{
	long	keyNumTypes;
	long	keyNumRecords;
	long	keyFileSize;
	long	keyCompressedSize;
};

class DTSKeyFilePriv;

class DTSKeyFile
{
public:
	DTSImplementNoCopy<DTSKeyFilePriv> priv;
	
	// interface
	DTSError	Open( const char * fname, uint flags, int fTypeID = 0, uint inNumEntries = 0 );
	DTSError	Open( DTSFileSpec * spec, uint flags, int fTypeID = 0, uint inNumEntries = 0 );
	
	void		Close();
	
					// returns noErr if the specified entry exists, else -1
	DTSError	Exists( DTSKeyType type, DTSKeyID id ) const;
	
	DTSError	ReadAlloc( DTSKeyType type, DTSKeyID id, void *& oBuffer );
	DTSError	GetSize( DTSKeyType type, DTSKeyID id, size_t * oSize ) const;
	DTSError	Read( DTSKeyType type, DTSKeyID id, void * buffer, size_t bufsize = ULONG_MAX );
	DTSError	Write( DTSKeyType type, DTSKeyID id, const void * buffer, size_t size );
	DTSError	Delete( DTSKeyType type, DTSKeyID id );
	DTSError	Compress();
	DTSError	Count( DTSKeyType type, long * oCount ) const;
	DTSError	GetID( DTSKeyType type, long index, DTSKeyID * oID ) const;
	DTSError	GetIndex( DTSKeyType type, DTSKeyID id, long * oIndex ) const;
	DTSError	GetUnusedID( DTSKeyType type, DTSKeyID * oID ) const;
	DTSError	GetInfo( DTSKeyInfo * oInfo ) const;
	DTSError	CountTypes( long * oCount ) const;
	DTSError	GetType( long index, DTSKeyType * oType ) const;
	int			SetWriteMode( int mode );		// returns previous write mode
	int			GetWriteMode() const;
};

enum		// flags for Open()
{
	kKeyReadOnlyPerm	= 0x00,
	kKeyReadWritePerm	= 0x01,
	kKeyCreateFile		= 0x00,
	kKeyDontCreateFile	= 0x02
};

enum		// modes for Get/SetWriteMode()
{
	kWriteModeReliable,
	kWriteModeFaster
};


#endif  // KeyFile_dts_h
