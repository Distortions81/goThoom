/*
**	DatabaseTypes_cl.h		ClanLord, CLServer
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

#ifndef DatabaseTypes_cl_h
#define	DatabaseTypes_cl_h

// NB: this file is read by the C++ _and_ the Rez compilers, so you
// must remain compatible with both.  The trickiest part of that is
// with hex escapes in character-constants and strings:
// C/C++ want "\xNN", but Rez wants "\0xNN" (or "\$NN").  Sigh.
// This pretty much means you have to fall back to octal notation.


// use symbols for the key file database types

// since i changed the format for the picture definitions
// i had to give them a new id so i could tell the old ones
// from the new ones.
// only the editor will use the old picture definition type
// and then only to update the data to the new format

// data types that are thoroughly and totally obsolete
#if 0
#define		kTypeAreaDoorsOld			'AreD'
#define		kTypeAreaDoorNamesOld		'ArDN'
#define		kTypeAreaNameOld			'AreN'
#define		kTypeAreaSignsOld			'AreS'
#define		kTypeAreaSigns				'Sign'
#endif  // 0

// types that are potentially still in use.
// It's important that these values never change.
#define		kTypeAccountOld				'Acct'
#define		kTypeAccountOld2			'Act2'
#define		kTypeAreaCrittersOld		'AreC'
#define		kTypeAreaDataOld			'AreI'
#define		kTypeAreaDataOld2			'ArI2'
#define		kTypeAreaDataOld3			'ArI3'
#define		kTypeAreaDataOld4			'ArI4'
#define		kTypeAreaDataOld5			'ArI5'
#define		kTypeAreaDataOld6			'ArI6'
#define		kTypeAreaDataOld7			'ArI7'
#define		kTypeAreaDataOld8			'ArI8'
#define		kTypeAreaMasksOld			'AreM'
#define		kTypeAreaMasksOld2			'ArM2'
#define		kTypeAreaMasksOld3			'ArM3'
#define		kTypeAreaMasksOld4			'ArM4'
#define		kTypeAreaMasksOld5			'ArM5'
#define		kTypeAreaMasksOld6			'ArM6'
#define		kTypeAreaMasksOld7			'ArM7'
#define		kTypeAreaPicturesOld		'AreP'
#define		kTypeAreaPicturesOld2		'ArP2'
#define		kTypeAreaPicturesOld3		'ArP3'
#define		kTypeAreaNPCsOld			'ANpc'
#define		kTypeAreaNPCsOld2			'ANp2'
#define		kTypeAreaNPCsOld3			'ANp3'
#define		kTypeAreaNPCsOld4			'ANp4'
#define		kTypeAreaNPCsOld5			'ANp5'
#define		kTypeAreaNPCsOld6			'ANp6'
#define		kTypePictureBitsOld			'Bits'
#define		kTypePictureBitsOld2		'Bit2'
#define		kTypeClanOld				'Clan'
#define		kTypeClanOld2				'Cln2'
#define		kTypeClanDataOld			'CDat'
#define		kTypeClassOld				'Clss'
#define		kTypePictureColorsOld		'Clrs'
#define		kTypeItemOld				'Item'
#define		kTypeItemOld2				'Itm2'
#define		kTypeItemOld3				'Itm3'
#define		kTypeItemOld4				'Itm4'
#define		kTypeItemOld5				'Itm5'
#define		kTypeItemOld6				'Itm6'
#define		kTypeItemOld7				'Itm7'
#define		kTypeItemOld8				'Itm8'
#define		kTypeItemOld9				'Itm9'
#define		kTypeItemOldA				'ItmA'
#define		kTypeItemOldB				'ItmB'
#define		kTypeItemOldC				'ItmC'
#define		kTypeClientItemOld4			'CIm4'
#define		kTypeLayoutOld				'Layo'
#define		kTypeLayoutOld2				'Lay2'
#define		kTypeLayoutOld3				'Lay3'
#define		kTypeLightingDataOld		'Lit1'
#define		kTypeMachineIdentifierOld	'MacI'
#define		kTypeMonsterDefinitionOld	'MDef'
#define		kTypeMonsterDefinitionOld1	'MDf2'
#define		kTypeMonsterDefinitionOld2	'MDf3'
#define		kTypeMonsterDefinitionOld3	'MDf4'
#define		kTypeMonsterDefinitionOld4	'MDf5'
#define		kTypeMonsterDefinitionOld5	'MDf6'
#define		kTypeMonsterDefinitionOld7	'MDf7'
#define		kTypeMonsterDefinitionOld8	'MDf8'
#define		kTypeMonsterDefinitionOld9	'MDf9'
#define		kTypeMonsterDefinitionOld10	'MDfA'
#define		kTypeMonsterDefinitionOld11	'MDfB'
#define		kTypeMonsterGeneratorsOld	'MGen'
#define		kTypeMonsterGeneratorsOld2	'MGn2'
#define		kTypeMonsterGeneratorsOld3	'MGn3'
#define		kTypeMonsterListOld			'MLst'
#define		kTypeMonsterListOld2		'MLs2'
#define		kTypeMonsterListOld3		'MLs3'
#define		kTypeMonsterListOld4		'MLs4'
#define		kTypePersistentDataOld		'PDN '
#define		kTypePlayerDataOld			'PDat'
#define		kTypePlayerDataOld2			'PDt1'
#define		kTypePictureDefinitionOld	'PDef'
#define		kTypePictureDefinitionOld2	'PDf2'
#define		kTypePictureDefinitionOld3	'PDf3'
#define		kTypePictureDefinitionOld4	'PDf4'
#define		kTypePictureDefinitionOld5	'PDf5'
#define		kTypePictureNameOld			'PicN'
#define		kTypeRaceOld				'Race'
#define		kTypeRaceOld2				'Rac2'
#define		kTypeRaceOld3				'Rac3'
#define		kTypeSoundOld				'snd '
#define		kTypeSoundNameOld			'sndN'
#define		kTypeVersionOld				'Vers'

// "current" type names that are simply aliases to their versioned counterparts above
#define		kTypeAccount				kTypeAccountOld2
#define		kTypeAreaData				kTypeAreaDataOld7
#define		kTypeAreaMasks				kTypeAreaMasksOld7
#define		kTypeAreaPictures			kTypeAreaPicturesOld2
#define		kTypeAreaNPCs				kTypeAreaNPCsOld6
#define		kTypePictureBits			kTypePictureBitsOld2
#define		kTypeClan					kTypeClanOld2
#define		kTypeClanData				kTypeClanDataOld
#define		kTypeClass					kTypeClassOld
#define		kTypePictureColors			kTypePictureColorsOld
#define		kTypeItem					kTypeItemOldC
#define		kTypeClientItem				kTypeClientItemOld4
#define		kTypeLayout					kTypeLayoutOld3
#define		kTypeLightingData			kTypeLightingDataOld
#define		kTypeMachineIdentifier		kTypeMachineIdentifierOld
#define		kTypeMonsterDefinition		kTypeMonsterDefinitionOld11
#define		kTypeMonsterGenerators		kTypeMonsterGeneratorsOld3
#define		kTypeMonsterList			kTypeMonsterListOld4
#define		kTypePlayerData				kTypePlayerDataOld2
#define		kTypePictureDefinition		kTypePictureDefinitionOld5
#define		kTypePictureName			kTypePictureNameOld
#define		kTypePersistentData			kTypePersistentDataOld
#define		kTypePlayerDB				'Plyr'
#define		kTypeRace					kTypeRaceOld3
#define		kTypeSound					kTypeSoundOld
#define		kTypeSoundName				kTypeSoundNameOld
#define		kTypeVersion				kTypeVersionOld

// comments (\215 is the MacRoman 'cedilla' character)
#define		kTypeAreaComment			'ArI\215'
#define		kTypeClassComment			'Cls\215'
#define		kTypeItemComment			'Itm\215'
#define		kTypeMapLocationComment		'Geo\215'
#define		kTypeMonsterComment			'MDf\215'
#define		kTypeMonsterListComment		'MLs\215'
#define		kTypePictureComment			'Pic\215'
#define		kTypeRaceComment			'Rac\215'
#define		kTypeSoundComment			'snd\215'


// define the file references
enum
{
	rAppFREF				= 128,		// all 'APPL'
	rPrefsFREF				= 129,		// all 'pref'
	rClientImagesFREF		= 130,
	rClientSoundsFREF		= 131,

	rServerWorldFREF		= 134,
	rServerPlayersFREF		= 135,
	rServerBlockedFREF		= 136,

	rEditorUndoFREF			= 140,
	rEditorAreasFREF		= 141,
	rEditorDiffsFREF		= 142,
	rClientMovieFREF		= 143,
	rClientSignFREF			= 144,
	rServerSignFREF			= 145,
	rCompilerSignFREF		= 146,
	rEditorSignFREF			= 147,
	rClientPlayersFREF		= 148,
	rClientImagesUpdateFREF	= 149,
	rClientSoundsUpdateFREF	= 150,
	rEditorMarksFREF		= 151
};

#define		kClientPrefsFName			"CL_Prefs"
#define		kClientPlayersFName			"CL_Players"
#define		kClientImagesFName			"CL_Images"
#define		kClientSoundsFName			"CL_Sounds"
#define		kClientImagesUpdateFName	"CL_Images.update"
#define		kClientSoundsUpdateFName	"CL_Sounds.update"
#define		kServerPrefsFName			"CLS_Prefs"
#define		kServerWorldFName			"CLS_World"
#define		kServerPlayersFName			"CLS_Players"
#define		kServerBlockedPlayersFName	"CLS_Blocked"
#define		kServerStringsFName			"CLS_Strings"
#define		kCompilerPrefsFName			"CLC_Prefs"

#ifndef APPRENTICE
# define	kEditorPrefsFName			"CLE_Prefs"
# define	kEditorUndoFName			"CLE_Undo"
#else
# define	kEditorPrefsFName			"CLE_Apprentice_Prefs"
# define	kEditorUndoFName			"CLE_Apprentice_Undo"
#endif	// APPRENTICE

	// \306 is the MacRoman 'delta' character
#define		kClientAppSign				'CLc\306'
#define		kServerAppSign				'CLv\306'
#define		kCompilerAppSign			'CLo\306'
#define		kEditorAppSign				'CLe\306'


#endif	// DatabaseTypes_cl_h

