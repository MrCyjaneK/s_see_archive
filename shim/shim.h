#ifndef SPACESEE_ARCHIVE_SHIM_H
#define SPACESEE_ARCHIVE_SHIM_H

#ifdef __cplusplus
extern "C" {
#endif

int SPACESEE_archive_load(const char *workDir, const char *assetsDir);
const char *SPACESEE_archive_loadError(void);
void SPACESEE_archive_unload(void);
int SPACESEE_archive_isHeavy(void);
int SPACESEE_archive_assetCount(void);
const char *SPACESEE_archive_asset(int index);
const char *SPACESEE_archive_version(void);
const char *SPACESEE_archive_extension(void);
const char *SPACESEE_archive_filetypes(void);
long long SPACESEE_archive_maxInputSize(void);
long long SPACESEE_archive_analyze(const char *inputPath, const char *outputPath, const char *dataDir);
int SPACESEE_archive_progress(long long jobId, int scale);
const char *SPACESEE_archive_progressString(long long jobId);
void SPACESEE_archive_free(void *ptr);

#ifdef __cplusplus
}
#endif

#endif
