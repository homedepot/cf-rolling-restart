#!/bin/sh
set -e

PROJECT_DIR="$(cd "$(dirname "$0")/.."; pwd)"
MAC_FILENAME="rolling-restart-plugin-darwin"
LINUX_FILENAME32="rolling-restart-plugin-linux32"
LINUX_FILENAME64="rolling-restart-plugin-linux64"
WIN_FILENAME32="rolling-restart-plugin32.exe"
WIN_FILENAME64="rolling-restart-plugin64.exe"

if [[ "$1" = "release" ]] ; then
	TAG="$2"
	: ${TAG:?"Usage: build_all.sh [release] [TAG]"}


	if git tag | grep $TAG > /dev/null 2>&1 ; then
		echo "$TAG exists, remove it or increment"
		exit 1
	else
		MAJOR=`echo $TAG | sed 's/^v//' | awk 'BEGIN {FS = "." } ; { printf $1;}'`
		MINOR=`echo $TAG | sed 's/^v//' | awk 'BEGIN {FS = "." } ; { printf $2;}'`
		BUILD=`echo $TAG | sed 's/^v//' | awk 'BEGIN {FS = "." } ; { printf $3;}'`

		`sed -i "" -e "1,/Major:.*/s/Major:.*/Major: $MAJOR,/" \
			-e "1,/Minor:.*/s/Minor:.*/Minor: $MINOR,/" \
			-e "1,/Build:.*/s/Build:.*/Build: $BUILD,/" ${PROJECT_DIR}/rolling-restart.go`
	fi
fi

echo "Begin build process"

echo "- Building for OSX"
GOOS=darwin GOARCH=amd64 go build -o $MAC_FILENAME
OSX_SHA1=`shasum -a 1 $MAC_FILENAME | awk '{print $1}'`
mkdir -p bin/osx
mv $MAC_FILENAME bin/osx

echo "- Building for WIN32"
GOOS=windows GOARCH=386 go build -o $WIN_FILENAME32
WIN32_SHA1=`shasum -a 1 $WIN_FILENAME32 | awk '{print $1}'`
mkdir -p bin/win32
mv $WIN_FILENAME32 bin/win32

echo "- Building for WIN64"
GOOS=windows GOARCH=amd64 go build -o $WIN_FILENAME64
WIN64_SHA1=`shasum -a 1 $WIN_FILENAME64 | awk '{print $1}'`
mkdir -p bin/win64
mv $WIN_FILENAME64 bin/win64

echo "- Building for LINUX32"
GOOS=linux GOARCH=386 go build -o $LINUX_FILENAME32
LINUX32_SHA1=`shasum -a 1 $LINUX_FILENAME32 | awk '{print $1}'`
mkdir -p bin/linux32
mv $LINUX_FILENAME32 bin/linux32

echo "- Building for LINUX64"
GOOS=linux GOARCH=amd64 go build -o $LINUX_FILENAME64
LINUX64_SHA1=`shasum -a 1 $LINUX_FILENAME64 | awk '{print $1}'`
mkdir -p bin/linux64
mv $LINUX_FILENAME64 bin/linux64

echo "End build process"

echo "Updating repo-index"
NOW=`TZ=UC date +'%Y-%m-%dT%TZ'`

cat repo-index.yml |
sed "s/__osx-sha1__/$OSX_SHA1/" |
sed "s/__win32-sha1__/$WIN32_SHA1/" |
sed "s/__win64-sha1__/$WIN64_SHA1/" |
sed "s/__linux32-sha1__/$LINUX32_SHA1/" |
sed "s/__linux64-sha1__/$LINUX64_SHA1/" |
sed "s/__TAG__/$TAG/" |
sed "s/__TODAY__/$NOW/" |
cat

if [[ "$1" = "release" ]] ; then
	git commit -am "Build version $TAG"
	git tag -a $TAG -m "Rolling Restart Plugin v$TAG"
	echo "Tagged release, 'git push --follow-tags' to push it to github, upload the binaries to github"
	echo "and copy the output above to the cli repo you plan to deploy in"
fi