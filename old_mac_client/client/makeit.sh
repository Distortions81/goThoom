#!/bin/bash -e
#
# A script to build the Clanlord client apps
#
#	Copyright 2023 Delta Tao Software, Inc.
# 
#	Licensed under the Apache License, Version 2.0 (the "License");
#	you may not use this file except in compliance with the License.
#	You may obtain a copy of the License at
# 
#		https://www.apache.org/licenses/LICENSE-2.0
# 
#	Unless required by applicable law or agreed to in writing, software
#	distributed under the License is distributed on an "AS IS" BASIS,
#	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#	See the License for the specific language governing permissions and
#	imitations under the License.

# pick one...
BUILDLOG=${BUILDLOG:=/tmp/CLClientBuild.log}
# BUILDLOG='Build Log'
# BUILDLOG=/dev/stdout

PROJECT='ClanLordX.xcodeproj'

# paranoia
if [ ! -d "$PROJECT" ]; then
	echo -e "\aCannot find ClanLord client Xcode project document!" 2>&1
	exit 1
fi

# fetch common configuration data
source ../makeit.config || { echo "couldn't find makeit.config" ; exit 1 ; } 2>&1

# fallback destination
DST_HOST=${DST_HOST:-"rer@big.local"}
#echo "Destination: $DST_HOST"


# this header file is the canonical source for the version number
VERS_FILE=public/VersionNumber_cl.h
if [ ! -r "$VERS_FILE" ]; then
	echo -e "\aCannot find client/$VERS_FILE!" 2>&1
	exit 1
fi

# deduce the current version numbers
BASEVERS=$(awk '/^#define kBaseVersionNumber/ { print $3 }' "$VERS_FILE")
SUB_VERS=$(awk '/^#define kSubVersionNumber/  { print $3 }' "$VERS_FILE")
if [ -z "$BASEVERS" -o -z "$SUB_VERS" ]; then
	echo -e "\aCannot determine version number!" 2>&1
	exit 1
fi

# only non-zero sub-version numbers shown
if (( ! SUB_VERS )) ; then
	VERSION="$BASEVERS"
else
	VERSION="${BASEVERS}.${SUB_VERS}"
fi

echo "Building Client apps v${VERSION} -- $(date)" > "$BUILDLOG"
echo "Building Client apps v${VERSION}"

CheckWinInstruments ()
{
	local MAC_BARD_FILE="source/Resources/ClanLord.r"
	local WIN_BARD_FILE="../winclient/client/src/sound/QuickTime/CLInstruments.cpp"
	
	if [ "$MAC_BARD_FILE" -nt "$WIN_BARD_FILE" ]; then
		echo "# *** Note, Win client bard instruments are out-of-date. " \
			 "Run export_instruments"
	fi
}

CheckWinInstruments

# clean things up before the production build
echo '# Cleaning all targets...'
xcodebuild -project "$PROJECT" -alltargets -configuration Debug   clean	>> "$BUILDLOG"
xcodebuild -project "$PROJECT" -alltargets -configuration Release clean	>> "$BUILDLOG"

# build the apps!

echo '# Building Release client...'
if ! xcodebuild -project "$PROJECT" -target ClanLordX 	\
	-configuration Release								\
	ARCHS='i386 x86_64'									\
	build >> "$BUILDLOG"
then
	echo -e "\aFailed to build Release client!" 2>&1
	exit 1
fi

echo '# Building Debug client...'
if ! xcodebuild -project "$PROJECT" -target ClanLordX	\
	-configuration Debug								\
	ARCHS='i386 x86_64'									\
	ONLY_ACTIVE_ARCH=NO									\
	build >> "$BUILDLOG"
then
	echo -e "\aFailed to build Debug client!" 2>&1
	exit 1
fi

# copy built apps out of the VM and into the host machine's filesystem
echo "# Exporting clients out of VM ..."
cd build || { echo "Can't find build folder!"; exit 1; } 2>&1

# Can't ssh/scp/rsync from a 10.6 host to a post-10.14 one... 
# use VMWare shared folder instead (if it works!)
if false; then
	SCP_DST="${DST_HOST}:Documents/Projects/clanlord/update/incoming/"
	scp -qCr "Debug/ClanLord+.app" "Release/ClanLord.app" $SCP_DST
else
	CP_DST='/Volumes/VMware Shared Folders/incoming'
	cp -a "Debug/ClanLord+.app" "Release/ClanLord.app" "$CP_DST"
fi

echo '# Done!'
