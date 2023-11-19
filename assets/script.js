$(document).ready(function() {
    $('[data-toggle="tooltip"]').tooltip();

    async function loadLikesForBook(bookID) {
        try {
            const response = await $.get(`/api/likes_count?book_id=${bookID}`);
            if (typeof response === 'object' && response.hasOwnProperty('count')) {
                return response.count;
            } else {
                console.error("Invalid response, error fetching likes count", response);
                return null;
            }
        } catch (error) {
            console.error("Error fetching likes count", error);
            return null;
        }
    }

    $('.badge[data-book-id]').each(async function() {
        const badgeElement = $(this);
        const bookID = badgeElement.data('book-id');
        const count = await loadLikesForBook(bookID);
        if (count !== null) {
            badgeElement.text(count);
        } else {
            console.error("Error loading likes for book ID:", bookID);
        }
    });

    $('#bookForm').on('submit', function(e) {
        e.preventDefault();

        var formData = new FormData(this);

        $.ajax({
            url: '/addbook',
            type: 'POST',
            data: formData,
            contentType: false,
            processData: false,
            success: function(response) {
                console.log('Libro agregado con éxito', response);
            },
            error: function(xhr, status, error) {
                console.error('Error al agregar el libro:', error);
            }
        });
    });

    $('.like-emoji').click(async function() {
        const clickedElement = $(this);
        const bookID = clickedElement.data('book-id');
        console.log('The book ID is: ');
        console.log(bookID);
        window.bookIdForRecaptcha = bookID;
        const siteKey = $('#captcha-container').data('sitekey');
        try {
            const response = await $.get(`/api/check_like/${bookID}`);
            console.log(response);
            switch (response.status) {
                case "unauthenticated":
                    const infoModal = clickedElement.siblings('.info-modal');
                    infoModal.text('Ingresa al sitio primero');
                    infoModal.show();
                    setTimeout(() => infoModal.hide(), 3000);
                    break;

                case "liked":
                    try {
                        await $.ajax({
                            url: '/api/like',
                            type: 'DELETE',
                            data: JSON.stringify({ book_id: bookID.toString() }),
                            contentType: 'application/json'
                        });
                        clickedElement.removeClass('active');
                        clickedElement.attr('data-original-title', 'Dar like');
                        console.log('Updating word like count after unliking word');
                        await updateBadgeCount(bookID);
                    } catch (error) {
                        console.log('Got an error: ');
                        console.log(error);
                        if (error.status >= 500 && error.status < 600) {
                            console.error("Server error:", error.statusText);
                        }
                    }
                    break;

                case "not-liked":
                    try {
                        await $.ajax({
                            url: '/api/like',
                            type: 'POST',
                            data: { book_id: bookID }
                        });

                        // Si la petición es exitosa, ejecutar el siguiente código
                        clickedElement.addClass('active');
                        clickedElement.attr('data-original-title', 'Quitar like');
                        console.log('Updating word like count after liking word.');
                        await updateBadgeCount(bookID);

                    } catch (error) {
                        // Manejo de errores
                        console.log('Got an error: ', error);
                        if (error && error.status >= 500 && error.status < 600) {
                            console.error("Server error:", error.statusText);
                        }
                    }

                    break;
            }
        } catch (error) {
            if (error.status >= 500 && error.status < 600) {
                const errorModal = clickedElement.siblings('.error-modal');
                errorModal.show();
                setTimeout(() => errorModal.hide(), 3000);
            }
        }

        // try {
        //     const response = await $.get(`/api/check_like/${bookID}`);
        //     console.log('Response: ');
        //     console.log(response);
        // } catch (error) {
        //     console.log('Error: ');
        //     console.log(error);
        //     if (error.status >= 500 && error.status < 600) {
        //         const errorModal = clickedElement.siblings('.error-modal');
        //         errorModal.show();
        //         setTimeout(() => errorModal.hide(), 3000);
        //     }
        // }
        //
        // $('#captcha-container').html('<div class="g-recaptcha" data-sitekey="' + siteKey + '" data-callback="onRecaptchaSuccess"></div>');
        // grecaptcha.render($('.g-recaptcha')[0]); // Renderiza el widget de reCAPTCHA
        // $('#captcha-container').show();
    });

    window.onRecaptchaSuccess = async function(token) {
        const bookID = window.bookIdForRecaptcha;
        try {
            console.log('Trying to like: ', bookID);
            await $.post('/api/like', { book_id: bookID });
            const clickedElement = $('.like-emoji[data-book-id="' + bookID + '"]');
            clickedElement.addClass('active');
            clickedElement.attr('data-original-title', 'Quitar like');
            await updateBadgeCount(bookID);
        } catch (error) {
            console.log('Got an error: ', error);
            if (error.status >= 500 && error.status < 600) {
                console.error("Server error:", error.statusText);
            }
        }
    };

    $('#searchForm').submit(function(e) {
        e.preventDefault();
        const textToSearch = $('#textSearch').val().trim();

        if (textToSearch) {
            const searchTypes = $("input[name='searchType']:checked").map(function() {
                return $(this).val();
            }).get();
            window.location.href = `search_books?textSearch=${textToSearch}&searchType=${searchTypes.join(',')}`;
        } else {
            $('.error-message').show();
        }
    });

    $('input[type="checkbox"][name="author"]').change(function() {
        $('input[type="checkbox"][name="author"]').prop('checked', false);
        $(this).prop('checked', true);
    });

    $("input[name='author']").change(async function() {
        const author = $(this).val();
        if (author) {
            $("#booksList").empty();
            try {
                const books = await $.get(`/api/books?start_with=${author}`);
                console.log(books);

                books.forEach(book => {
                    $("#booksList").append(`
                        <div class="card my-2">
                            <div class="card-body">
                                <h5 class="card-title"><a href="book_info?id=${book.id}">${book.title}</a></h5>
                                <h6 class="card-subtitle mb-2 text-muted">${book.author}</h6>
                                <p class="card-text">${book.description || ""}</p>
                            </div>                            
                            <img src="${book.image}" class="card-img-bottom" alt="Image of ${book.title}">
                        </div>
                    `);
                });
            } catch (error) {
                console.error("Error fetching books by author:", error);
            }
        }
    });

    async function updateBadgeCount(bookID) {
        console.log('Book ID to update: ' + bookID);
        const count = await loadLikesForBook(bookID);
        const badgeElement = $(`.badge[data-book-id=${bookID}]`);
        if (count !== null) {
            badgeElement.text(count);
        } else {
            console.error("Error updating likes for book ID:", bookID);
        }
    }

    (async function() {
        try {
            const booksCountData = await $.get(`/api/booksCount`);
            $('#booksCount').text(booksCountData.booksCount);
        } catch (error) {
            console.error("Error fetching books count:", error);
        }
    })();
});
