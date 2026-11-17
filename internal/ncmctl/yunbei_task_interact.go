// Copyright (c) 2026 chaunsin
// SPDX-License-Identifier: MIT

package ncmctl

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/chaunsin/netease-cloud-music/api/eapi"
	apitypes "github.com/chaunsin/netease-cloud-music/api/types"
	"github.com/chaunsin/netease-cloud-music/api/weapi"
)

func (runtime *yunBeiRuntime) loadComments(ctx context.Context, threadID string, commentID int64) (*weapi.CommentsResp, error) {
	req := &weapi.CommentsReq{
		ThreadId:   threadID,
		Offset:     "0",
		Limit:      "20",
		BeforeTime: "0",
	}
	if commentID > 0 {
		req.CommentId = strconv.FormatInt(commentID, 10)
	}

	reply, err := runtime.weapi.Comments(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("Comments(%s): %w", threadID, err)
	}

	if reply.Code != 200 {
		return nil, fmt.Errorf("Comments(%s): code=%d", threadID, reply.Code)
	}
	return reply, nil
}

func (runtime *yunBeiRuntime) restoreComment(ctx context.Context, threadID string, commentID int64) error {
	reply, err := runtime.weapi.CommentUnlike(ctx, &weapi.CommentLikeReq{
		ThreadId:  threadID,
		CommentId: strconv.FormatInt(commentID, 10),
	})
	if err != nil {
		return fmt.Errorf("CommentUnlike(%d): %w", commentID, err)
	}

	if reply.Code != 200 {
		return fmt.Errorf("CommentUnlike(%d): code=%d message=%q", commentID, reply.Code, reply.Message)
	}

	return runtime.pollRestore(ctx, "comment restore", func(ctx context.Context) (bool, error) {
		comments, err := runtime.loadComments(ctx, threadID, commentID)
		if err != nil {
			return false, err
		}

		for commentIndex := range comments.Comments {
			comment := &comments.Comments[commentIndex]
			if comment.CommentId == commentID && !comment.Liked {
				return true, nil
			}
		}
		return false, nil
	}, fmt.Errorf("CommentUnlike(%d): restored state was not confirmed", commentID))
}

func runYunBeiLikeComment(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	if err := validateYunBeiActionableTask(snapshot); err != nil {
		return err
	}

	recommend, err := runtime.weapi.RecommendSongs(ctx, &weapi.RecommendSongsReq{})
	if err != nil {
		return fmt.Errorf("RecommendSongs: %w", err)
	}

	if recommend.Code != 200 {
		return fmt.Errorf("RecommendSongs: code=%d message=%q", recommend.Code, recommend.Message)
	}

	var actions int

	for songIndex := range recommend.Data.DailySongs {
		song := &recommend.Data.DailySongs[songIndex]
		if song.Id <= 0 {
			continue
		}

		threadID := "R_SO_4_" + strconv.FormatInt(song.Id, 10)

		comments, err := runtime.loadComments(ctx, threadID, 0)
		if err != nil {
			return err
		}

		for commentIndex := range comments.Comments {
			comment := &comments.Comments[commentIndex]
			if comment.CommentId <= 0 || comment.User.UserId <= 0 || comment.Liked || comment.User.UserId == runtime.uid {
				continue
			}

			key := yunBeiActionKey{
				TaskID:   yunBeiLikeComment,
				Kind:     "like",
				TargetID: threadID + ":" + strconv.FormatInt(comment.CommentId, 10),
			}
			if !runtime.beginAction(key) {
				continue
			}

			liked, err := runtime.weapi.CommentLike(ctx, &weapi.CommentLikeReq{
				ThreadId:  threadID,
				CommentId: strconv.FormatInt(comment.CommentId, 10),
			})
			if err != nil {
				return fmt.Errorf("CommentLike(%d): %w", comment.CommentId, err)
			}

			if liked.Code != 200 {
				return fmt.Errorf("CommentLike(%d): code=%d message=%q", comment.CommentId, liked.Code, liked.Message)
			}

			commentID := comment.CommentId
			runtime.addCompensation(
				yunBeiLikeComment,
				"comment-unlike",
				strconv.FormatInt(commentID, 10),
				threadID,
				func(ctx context.Context) error {
					return runtime.restoreComment(ctx, threadID, commentID)
				},
			)

			actions++

			runtime.printf("  点赞评论: 歌曲 %s (%d)，评论ID %d\n", song.Name, song.Id, commentID)

			confirmed, err := runtime.observeTask(ctx, snapshot, yunBeiQuickObserve)
			if err != nil {
				return err
			}

			if !confirmed {
				return nil
			}

			if snapshot.State != yunBeiCompletedUnclaimed && snapshot.State != yunBeiClaimed {
				continue
			}

			if err := runtime.restoreAndVerify(ctx, snapshot, 0, yunBeiLikeComment); err != nil {
				return err
			}
			return nil
		}
	}

	if actions == 0 && !runtime.attemptedTask(yunBeiLikeComment) {
		snapshot.State = yunBeiUnavailable
	}
	return nil
}

func yunBeiPlaylistOwnedAndOrdinary(userID, playlistID, ownerID, creatorID, specialType int64) bool {
	return playlistID > 0 && ownerID == userID && creatorID == userID && specialType == 0
}

func yunBeiPlaylistContainsSong(detail *weapi.PlaylistDetailResp, songID int64) bool {
	for trackIndex := range detail.Playlist.TrackIds {
		if detail.Playlist.TrackIds[trackIndex].Id == songID {
			return true
		}
	}
	return false
}

func (runtime *yunBeiRuntime) loadPlaylistDetail(ctx context.Context, playlistID int64) (*weapi.PlaylistDetailResp, error) {
	reply, err := runtime.weapi.PlaylistDetail(ctx, &weapi.PlaylistDetailReq{
		Id: strconv.FormatInt(playlistID, 10),
	})
	if err != nil {
		return nil, fmt.Errorf("PlaylistDetail(%d): %w", playlistID, err)
	}

	if reply.Code != 200 {
		return nil, fmt.Errorf("PlaylistDetail(%d): code=%d", playlistID, reply.Code)
	}

	if reply.Playlist.TrackCount < 0 || reply.Playlist.TrackCount > int64(len(reply.Playlist.TrackIds)) {
		return nil, fmt.Errorf(
			"PlaylistDetail(%d): incomplete trackIds count=%d expected=%d",
			playlistID,
			len(reply.Playlist.TrackIds),
			reply.Playlist.TrackCount,
		)
	}
	return reply, nil
}

func (runtime *yunBeiRuntime) restorePlaylistSong(ctx context.Context, playlistID, songID int64) error {
	reply, err := runtime.weapi.PlaylistAddOrDel(ctx, &weapi.PlaylistAddOrDelReq{
		Op:       "del",
		Pid:      playlistID,
		TrackIds: apitypes.IntsString{songID},
		Imme:     true,
	})
	if err != nil {
		return fmt.Errorf("PlaylistAddOrDel(del,%d,%d): %w", playlistID, songID, err)
	}

	if reply.Code != 200 {
		return fmt.Errorf("PlaylistAddOrDel(del,%d,%d): code=%d message=%q", playlistID, songID, reply.Code, reply.Message)
	}

	return runtime.pollRestore(ctx, "playlist restore", func(ctx context.Context) (bool, error) {
		detail, err := runtime.loadPlaylistDetail(ctx, playlistID)
		if err != nil {
			return false, err
		}
		return !yunBeiPlaylistContainsSong(detail, songID), nil
	}, fmt.Errorf("PlaylistAddOrDel(del,%d,%d): restored state was not confirmed", playlistID, songID))
}

func runYunBeiCollectSong(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	if err := validateYunBeiActionableTask(snapshot); err != nil {
		return err
	}

	playlists, err := runtime.weapi.Playlist(ctx, &weapi.PlaylistReq{
		Uid:   strconv.FormatInt(runtime.uid, 10),
		Limit: "100",
	})
	if err != nil {
		return fmt.Errorf("playlist: %w", err)
	}

	if playlists.Code != 200 {
		return fmt.Errorf("playlist: code=%d message=%q", playlists.Code, playlists.Message)
	}

	recommend, err := runtime.weapi.RecommendSongs(ctx, &weapi.RecommendSongsReq{})
	if err != nil {
		return fmt.Errorf("RecommendSongs: %w", err)
	}

	if recommend.Code != 200 {
		return fmt.Errorf("RecommendSongs: code=%d message=%q", recommend.Code, recommend.Message)
	}

	var (
		actions int
		lastErr error
	)

	for playlistIndex := range playlists.Playlist {
		playlist := &playlists.Playlist[playlistIndex]
		if !yunBeiPlaylistOwnedAndOrdinary(
			runtime.uid,
			playlist.Id,
			playlist.UserId,
			playlist.Creator.UserId,
			playlist.SpecialType,
		) {
			continue
		}

		detail, err := runtime.loadPlaylistDetail(ctx, playlist.Id)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			runtime.l.Debugf("[yunbei] collect-song playlist %d detail: %v", playlist.Id, err)
			lastErr = err
			continue
		}

		if !yunBeiPlaylistOwnedAndOrdinary(
			runtime.uid,
			detail.Playlist.Id,
			detail.Playlist.UserId,
			detail.Playlist.Creator.UserId,
			detail.Playlist.SpecialType,
		) {
			continue
		}

		for songIndex := range recommend.Data.DailySongs {
			song := &recommend.Data.DailySongs[songIndex]
			if song.Id <= 0 || yunBeiPlaylistContainsSong(detail, song.Id) {
				continue
			}

			key := yunBeiActionKey{
				TaskID:   yunBeiCollectSong,
				Kind:     "collect",
				TargetID: fmt.Sprintf("%d:%d", playlist.Id, song.Id),
			}
			if !runtime.beginAction(key) {
				continue
			}

			added, err := runtime.weapi.PlaylistAddOrDel(ctx, &weapi.PlaylistAddOrDelReq{
				Op:       "add",
				Pid:      playlist.Id,
				TrackIds: apitypes.IntsString{song.Id},
				Imme:     true,
			})
			if err != nil {
				return fmt.Errorf("PlaylistAddOrDel(add,%d,%d): %w", playlist.Id, song.Id, err)
			}

			if added.Code != 200 {
				return fmt.Errorf(
					"PlaylistAddOrDel(add,%d,%d): code=%d message=%q",
					playlist.Id,
					song.Id,
					added.Code,
					added.Message,
				)
			}

			playlistID, songID := playlist.Id, song.Id
			runtime.addCompensation(
				yunBeiCollectSong,
				"playlist-remove-song",
				strconv.FormatInt(songID, 10),
				strconv.FormatInt(playlistID, 10),
				func(ctx context.Context) error {
					return runtime.restorePlaylistSong(ctx, playlistID, songID)
				},
			)

			actions++

			runtime.printf(
				"  收藏歌曲: %s (%d) -> %s (%d)\n",
				song.Name,
				songID,
				playlist.Name,
				playlistID,
			)

			confirmed, err := runtime.observeTask(ctx, snapshot, yunBeiQuickObserve)
			if err != nil {
				return err
			}

			if !confirmed {
				return nil
			}

			if snapshot.State != yunBeiCompletedUnclaimed && snapshot.State != yunBeiClaimed {
				continue
			}

			if err := runtime.restoreAndVerify(ctx, snapshot, 0, yunBeiCollectSong); err != nil {
				return err
			}
			return nil
		}
	}

	if actions == 0 && !runtime.attemptedTask(yunBeiCollectSong) {
		if lastErr != nil {
			return lastErr
		}

		snapshot.State = yunBeiUnavailable
	}
	return nil
}

func (runtime *yunBeiRuntime) loadHotArtists(ctx context.Context) (*eapi.ArtistHotResp, error) {
	reply, err := runtime.eapi.ArtistHot(ctx, &eapi.ArtistHotReq{Offset: 0, Limit: 30})
	if err != nil {
		return nil, fmt.Errorf("ArtistHot: %w", err)
	}

	if reply.Code != 200 {
		return nil, fmt.Errorf("ArtistHot: code=%d", reply.Code)
	}
	return reply, nil
}

func (runtime *yunBeiRuntime) artistFollowed(ctx context.Context, artistID int64) (bool, error) {
	offset := int64(0)

	for range yunBeiArtistMaxPages {
		subscriptions, err := runtime.weapi.ArtistSublist(ctx, &weapi.ArtistSublistReq{
			Offset: offset,
			Limit:  yunBeiArtistPageSize,
			Total:  true,
		})
		if err != nil {
			return false, fmt.Errorf("ArtistSublist: %w", err)
		}

		if subscriptions.Code != 200 {
			return false, fmt.Errorf("ArtistSublist: code=%d message=%q", subscriptions.Code, subscriptions.Message)
		}

		for _, artist := range subscriptions.Data {
			if artist.Id == artistID {
				return true, nil
			}
		}

		nextOffset := offset + int64(len(subscriptions.Data))
		more := subscriptions.HasMore || subscriptions.Count > nextOffset

		if !more {
			return false, nil
		}

		if nextOffset == offset {
			return false, errors.New("ArtistSublist: response indicates another page but returned no artists")
		}

		offset = nextOffset
	}

	return false, fmt.Errorf("ArtistSublist: exceeded %d pages", yunBeiArtistMaxPages)
}

func (runtime *yunBeiRuntime) restoreArtist(ctx context.Context, artistID int64) error {
	reply, err := runtime.eapi.ArtistUnsub(ctx, &eapi.ArtistUnsubReq{
		ArtistIds: strconv.FormatInt(artistID, 10),
	})
	if err != nil {
		return fmt.Errorf("ArtistUnsub: %w", err)
	}

	if reply.Code != 200 {
		return fmt.Errorf("ArtistUnsub: code=%d message=%q", reply.Code, reply.Message)
	}

	return runtime.pollRestore(ctx, "artist restore", func(ctx context.Context) (bool, error) {
		followed, err := runtime.artistFollowed(ctx, artistID)
		if err != nil {
			return false, err
		}
		return !followed, nil
	}, fmt.Errorf("ArtistUnsub(%d): restored state was not confirmed", artistID))
}

func runYunBeiFollowArtist(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	if err := validateYunBeiActionableTask(snapshot); err != nil {
		return err
	}

	hot, err := runtime.loadHotArtists(ctx)
	if err != nil {
		return err
	}

	var candidate *eapi.ArtistHotRespArtist

	for groupIndex := range hot.Data {
		for artistIndex := range hot.Data[groupIndex].Artists {
			artist := &hot.Data[groupIndex].Artists[artistIndex]
			if artist.Id <= 0 || artist.Followed {
				continue
			}

			key := yunBeiActionKey{
				TaskID:   yunBeiFollowArtist,
				Kind:     "follow",
				TargetID: strconv.FormatInt(artist.Id, 10),
			}
			if runtime.beginAction(key) {
				candidate = artist
				break
			}
		}

		if candidate != nil {
			break
		}
	}

	if candidate == nil {
		if !runtime.attemptedTask(yunBeiFollowArtist) {
			snapshot.State = yunBeiUnavailable
		}
		return nil
	}

	artistID := candidate.Id
	artistName := candidate.Name
	compensationStart := len(runtime.compensations)

	followed, err := runtime.eapi.ArtistSub(ctx, &eapi.ArtistSubReq{ArtistId: strconv.FormatInt(artistID, 10)})
	if err != nil {
		return fmt.Errorf("ArtistSub(%d): %w", artistID, err)
	}

	if followed.Code != 200 {
		return fmt.Errorf("ArtistSub(%d): code=%d message=%q", artistID, followed.Code, followed.Message)
	}

	runtime.addCompensation(
		yunBeiFollowArtist,
		"artist-unsub",
		strconv.FormatInt(artistID, 10),
		"",
		func(ctx context.Context) error { return runtime.restoreArtist(ctx, artistID) },
	)
	runtime.printf("  关注歌手: %s (%d)\n", artistName, artistID)

	confirmed, err := runtime.observeTask(ctx, snapshot, yunBeiQuickObserve)
	if err != nil {
		return err
	}

	if !confirmed {
		return nil
	}

	if err := runtime.restoreAndVerify(ctx, snapshot, compensationStart); err != nil {
		return err
	}
	return nil
}

func (runtime *yunBeiRuntime) loadLikedSongs(ctx context.Context) (map[int64]struct{}, error) {
	reply, err := runtime.weapi.SongLikeList(ctx, &weapi.SongLikeListReq{Uid: runtime.uid})
	if err != nil {
		return nil, fmt.Errorf("SongLikeList: %w", err)
	}

	if reply.Code != 200 {
		return nil, fmt.Errorf("SongLikeList: code=%d message=%q", reply.Code, reply.Message)
	}

	liked := make(map[int64]struct{}, len(reply.Ids))
	for _, songID := range reply.Ids {
		if songID > 0 {
			liked[songID] = struct{}{}
		}
	}
	return liked, nil
}

func (runtime *yunBeiRuntime) setSongLike(ctx context.Context, songID int64, like bool, alg string) error {
	reply, err := runtime.weapi.SongLike(ctx, &weapi.SongLikeReq{
		TrackId: songID,
		Like:    like,
		Alg:     alg,
	})
	if err != nil {
		return fmt.Errorf("SongLike(%d,%t): %w", songID, like, err)
	}

	if reply.Code != 200 {
		return fmt.Errorf("SongLike(%d,%t): code=%d", songID, like, reply.Code)
	}
	return nil
}

func (runtime *yunBeiRuntime) restoreSongLike(ctx context.Context, songID int64, alg string) error {
	if err := runtime.setSongLike(ctx, songID, false, alg); err != nil {
		return err
	}

	return runtime.pollRestore(ctx, "song-like restore", func(ctx context.Context) (bool, error) {
		liked, err := runtime.loadLikedSongs(ctx)
		if err != nil {
			return false, err
		}

		_, exists := liked[songID]
		return !exists, nil
	}, fmt.Errorf("SongLike(%d,false): restored state was not confirmed", songID))
}

func runYunBeiLikeSong(ctx context.Context, runtime *yunBeiRuntime, snapshot *yunBeiTaskSnapshot) error {
	if err := validateYunBeiActionableTask(snapshot); err != nil {
		return err
	}

	liked, err := runtime.loadLikedSongs(ctx)
	if err != nil {
		return err
	}

	recommend, err := runtime.weapi.RecommendSongs(ctx, &weapi.RecommendSongsReq{})
	if err != nil {
		return fmt.Errorf("RecommendSongs: %w", err)
	}

	if recommend.Code != 200 {
		return fmt.Errorf("RecommendSongs: code=%d message=%q", recommend.Code, recommend.Message)
	}

	var (
		songID   int64
		songName string
		songAlg  string
	)

	for songIndex := range recommend.Data.DailySongs {
		song := &recommend.Data.DailySongs[songIndex]
		if song.Id <= 0 {
			continue
		}

		if _, alreadyLiked := liked[song.Id]; alreadyLiked {
			continue
		}

		key := yunBeiActionKey{
			TaskID:   yunBeiLikeSong,
			Kind:     "like",
			TargetID: strconv.FormatInt(song.Id, 10),
		}
		if runtime.beginAction(key) {
			songID = song.Id
			songName = song.Name
			songAlg = song.Alg
			break
		}
	}

	if songID == 0 {
		if !runtime.attemptedTask(yunBeiLikeSong) {
			snapshot.State = yunBeiUnavailable
		}
		return nil
	}

	compensationStart := len(runtime.compensations)

	if actionErr := runtime.setSongLike(ctx, songID, true, songAlg); actionErr != nil {
		return actionErr
	}

	runtime.addCompensation(
		yunBeiLikeSong,
		"song-unlike",
		strconv.FormatInt(songID, 10),
		"",
		func(ctx context.Context) error { return runtime.restoreSongLike(ctx, songID, songAlg) },
	)
	runtime.printf("  红心歌曲: %s (%d)\n", songName, songID)

	confirmed, err := runtime.observeTask(ctx, snapshot, yunBeiQuickObserve)
	if err != nil {
		return err
	}

	if !confirmed {
		return nil
	}

	if err := runtime.restoreAndVerify(ctx, snapshot, compensationStart); err != nil {
		return err
	}
	return nil
}
