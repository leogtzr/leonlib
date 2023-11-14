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

    function debounce(func, wait, immediate) {
        let timeout;
        return function() {
            const context = this, args = arguments;
            const later = function() {
                timeout = null;
                if (!immediate) func.apply(context, args);
            };
            const callNow = immediate && !timeout;
            clearTimeout(timeout);
            timeout = setTimeout(later, wait);
            if (callNow) func.apply(context, args);
        };
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

    $('.like-emoji').click(async function() {
        const clickedElement = $(this);
        const bookID = clickedElement.data('book-id');
        console.log('The book ID is: ');
        console.log(bookID);
        window.bookIdForRecaptcha = bookID;
        const siteKey = $('#captcha-container').data('sitekey'); // obtiene el SiteKey del atributo de datos

        $('#captcha-container').html('<div class="g-recaptcha" data-sitekey="' + siteKey + '" data-callback="onRecaptchaSuccess"></div>');
        grecaptcha.render($('.g-recaptcha')[0]); // Renderiza el widget de reCAPTCHA
        $('#captcha-container').show();
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
                console.log('1) Got: ');
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
        console.log('Count -> ' + count);
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
