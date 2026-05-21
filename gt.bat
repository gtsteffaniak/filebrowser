@echo off
if "%~1"=="" (
    echo Usage: gt "commit message"
    exit /b 1
)
 
git add . || exit /b 1
git commit -m %1 || exit /b 1
git push || exit /b 1
 
echo Done.